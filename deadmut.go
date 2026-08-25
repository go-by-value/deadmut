// Package deadmut implements an analyzer that reports mutations of range loop
// value copies that have no effect.
package deadmut

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

const doc = `reports mutations of range loop value copies that have no effect

The value variable of a range loop over a slice, array, or map of structs is a
copy of the element. Writing to its fields, calling a pointer-receiver method
that writes to it, or passing its address to a function that writes through it
mutates the copy, not the element. When nothing reads the copy afterwards, the
mutation is dead.

deadmut tracks how functions write through their pointer receivers and pointer
parameters, also across packages, so that read-only methods such as Len or
String are not reported.`

// NewAnalyzer returns the deadmut analyzer.
func NewAnalyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:      "deadmut",
		Doc:       doc,
		URL:       "https://github.com/go-by-value/deadmut",
		Run:       run,
		Requires:  []*analysis.Analyzer{inspect.Analyzer},
		FactTypes: []analysis.Fact{(*writeFact)(nil)},
	}
}

// effect classifies what an operation does to the value a variable refers to.
type effect uint8

const (
	// effectNone means the value is only read.
	effectNone effect = iota
	// effectDirect means the operation writes to storage owned by the value:
	// its fields, nested structs, and arrays. Such writes are lost when the
	// value is a copy.
	effectDirect
	// effectShared means the operation may be observable beyond the value: it
	// writes through a pointer, slice, or map reachable from the value, lets
	// its address escape, or hands it to code we cannot see.
	effectShared
)

func (e effect) String() string {
	switch e {
	case effectNone:
		return "none"
	case effectDirect:
		return "direct"
	case effectShared:
		return "shared"
	}

	return fmt.Sprintf("effect(%d)", uint8(e))
}

// writeFact records how a function writes through its pointer receiver and its
// pointer parameters. Parameters that are not pointers always have effectNone.
type writeFact struct {
	Recv   effect
	Params []effect
}

func (*writeFact) AFact() {}

func (f *writeFact) String() string {
	parts := make([]string, 0, 1+len(f.Params))
	parts = append(parts, "recv="+f.Recv.String())
	for i, p := range f.Params {
		parts = append(parts, fmt.Sprintf("p%d=%s", i, p))
	}

	return strings.Join(parts, " ")
}

func (f *writeFact) param(i int) effect {
	if i < 0 || i >= len(f.Params) {
		return effectShared
	}

	return f.Params[i]
}

func (f *writeFact) isZero() bool {
	if f.Recv != effectNone {
		return false
	}
	for _, p := range f.Params {
		if p != effectNone {
			return false
		}
	}

	return true
}

func (f *writeFact) equal(g writeFact) bool {
	if f.Recv != g.Recv || len(f.Params) != len(g.Params) {
		return false
	}
	for i := range f.Params {
		if f.Params[i] != g.Params[i] {
			return false
		}
	}

	return true
}

type siteKind uint8

const (
	// siteRead: the root is read.
	siteRead siteKind = iota
	// siteWrite: an assignment or ++/-- targets storage reached from the root.
	siteWrite
	// siteMethodCall: a pointer-receiver method is called on storage reached
	// from the root.
	siteMethodCall
	// siteAddrCall: the address of storage reached from the root is passed to
	// a call.
	siteAddrCall
	// siteArgCall: the root itself, a pointer, is passed to a call.
	siteArgCall
	// siteEscape: the root, a pointer, is stored, returned, reassigned, or
	// otherwise leaves our sight.
	siteEscape
)

// site is one use of a root variable.
type site struct {
	kind siteKind
	// path is effectDirect when the path from the root to the used storage
	// stays within storage owned by the root, effectShared otherwise.
	path effect
	// callee is the called function for call kinds; nil when unknown.
	callee *types.Func
	// param is the parameter index for siteAddrCall and siteArgCall.
	param int
	// expr is the expression to report.
	expr ast.Expr
	// call is the call expression for call kinds.
	call *ast.CallExpr
	// stmt is the innermost enclosing statement.
	stmt ast.Stmt
	// resultUsed is true when the result of the call is consumed.
	resultUsed bool
	// deferred is true inside a defer or go statement or a function literal
	// within the owning range body.
	deferred bool
	// looped is true inside a loop nested within the owning range body.
	looped bool
}

// funcInfo is a function with pointer roots whose fact we compute.
type funcInfo struct {
	fn *types.Func
	// index maps each root to its parameter index; the receiver is -1.
	index map[types.Object]int
	sites map[types.Object][]site
	fact  writeFact
}

// loopInfo is a range loop whose value variable is a struct or array copy.
type loopInfo struct {
	stmt    *ast.RangeStmt
	v       *types.Var
	sites   []site
	hasGoto bool
}

type root struct {
	fn   *funcInfo
	loop *loopInfo
}

type analyzer struct {
	pass  *analysis.Pass
	funcs map[*types.Func]*funcInfo
	loops []*loopInfo
	roots map[types.Object]root
}

func run(pass *analysis.Pass) (any, error) {
	insp, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, fmt.Errorf("deadmut: unexpected result type for %s", inspect.Analyzer.Name)
	}

	a := analyzer{
		pass:  pass,
		funcs: map[*types.Func]*funcInfo{},
		roots: map[types.Object]root{},
	}
	a.collectRoots(insp)
	a.collectSites(insp)
	a.computeFacts()
	a.checkLoops()

	return nil, nil
}

func (a *analyzer) collectRoots(insp *inspector.Inspector) {
	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil), (*ast.RangeStmt)(nil)}, func(n ast.Node) {
		switch n := n.(type) {
		case *ast.FuncDecl:
			a.collectFuncRoots(n)
		case *ast.RangeStmt:
			a.collectLoopRoots(n)
		}
	})
}

func (a *analyzer) collectFuncRoots(decl *ast.FuncDecl) {
	if decl.Body == nil {
		return
	}
	fn, ok := a.pass.TypesInfo.Defs[decl.Name].(*types.Func)
	if !ok {
		return
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return
	}

	fi := funcInfo{
		fn:    fn,
		index: map[types.Object]int{},
		sites: map[types.Object][]site{},
		fact:  writeFact{Params: make([]effect, sig.Params().Len())},
	}
	if recv := sig.Recv(); recv != nil && isPointer(recv.Type()) {
		fi.index[recv] = -1
	}
	for i := range sig.Params().Len() {
		p := sig.Params().At(i)
		if isPointer(p.Type()) {
			fi.index[p] = i
		}
	}
	if len(fi.index) == 0 {
		return
	}

	a.funcs[fn] = &fi
	for obj := range fi.index {
		a.roots[obj] = root{fn: &fi}
	}
}

func (a *analyzer) collectLoopRoots(rs *ast.RangeStmt) {
	for _, ident := range a.copyVars(rs) {
		v, ok := a.pass.TypesInfo.Defs[ident].(*types.Var)
		if !ok || !isCopyType(v.Type()) {
			continue
		}

		li := loopInfo{stmt: rs, v: v, hasGoto: hasGoto(rs.Body)}
		a.loops = append(a.loops, &li)
		a.roots[v] = root{loop: &li}
	}
}

// copyVars returns the loop variables of rs that receive a copy of an element.
func (a *analyzer) copyVars(rs *ast.RangeStmt) []*ast.Ident {
	if rs.Tok != token.DEFINE {
		return nil
	}
	x := a.pass.TypesInfo.TypeOf(rs.X)
	if x == nil {
		return nil
	}

	switch t := under(x).(type) {
	case *types.Slice, *types.Array, *types.Map:
		return idents(rs.Value)
	case *types.Pointer:
		if _, ok := under(t.Elem()).(*types.Array); ok {
			return idents(rs.Value)
		}
	case *types.Chan:
		return idents(rs.Key)
	case *types.Signature:
		return idents(rs.Key, rs.Value)
	}

	return nil
}

func (a *analyzer) collectSites(insp *inspector.Inspector) {
	insp.WithStack([]ast.Node{(*ast.Ident)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return false
		}
		ident, ok := n.(*ast.Ident)
		if !ok {
			return false
		}
		obj := a.pass.TypesInfo.Uses[ident]
		if obj == nil {
			return false
		}
		r, ok := a.roots[obj]
		if !ok {
			return false
		}

		if r.fn != nil {
			s := a.classify(stack, true)
			r.fn.sites[obj] = append(r.fn.sites[obj], s)

			return false
		}

		s := a.classify(stack, false)
		s.deferred, s.looped = loopContext(stack, r.loop.stmt)
		r.loop.sites = append(r.loop.sites, s)

		return false
	})
}

// classify describes how the identifier at the top of stack uses its root.
// pointerRoot is true when the root variable is a pointer.
func (a *analyzer) classify(stack []ast.Node, pointerRoot bool) site {
	info := a.pass.TypesInfo

	i := len(stack) - 1
	loc, _ := stack[i].(ast.Expr)
	derefs := 0
	shared := false
	plain := true
	var method *types.Func

walk:
	for i > 0 {
		switch p := stack[i-1].(type) {
		case *ast.ParenExpr:
			loc = p
		case *ast.SelectorExpr:
			if p.X != loc {
				break walk
			}
			sel := info.Selections[p]
			if sel == nil {
				break walk
			}
			d, ok := selectionDerefs(sel)
			if !ok {
				shared = true
			}
			derefs += d
			plain = false
			loc = p
			if sel.Kind() != types.FieldVal {
				method, _ = sel.Obj().(*types.Func)
				i--

				break walk
			}
		case *ast.IndexExpr:
			if p.X != loc {
				break walk
			}
			switch under(info.TypeOf(p.X)).(type) {
			case *types.Array:
			case *types.Pointer:
				derefs++
			default:
				shared = true
			}
			plain = false
			loc = p
		case *ast.StarExpr:
			derefs++
			plain = false
			loc = p
		default:
			break walk
		}
		i--
	}

	s := site{kind: siteRead, expr: loc, path: effectDirect, stmt: enclosingStmt(stack[:i])}
	if shared {
		s.path = effectShared
	}
	if pointerRoot {
		if derefs > 1 {
			s.path = effectShared
		}
	} else if derefs > 0 {
		s.path = effectShared
	}

	parent := stack[i-1]
	if method != nil {
		call, ok := parent.(*ast.CallExpr)
		if !ok || call.Fun != loc {
			// A method value bound to a pointer receiver takes the address.
			if hasPointerReceiver(method) {
				s.kind = siteEscape
			}

			return s
		}
		if !hasPointerReceiver(method) {
			return s
		}
		s.kind = siteMethodCall
		s.callee = method.Origin()
		s.call = call
		s.resultUsed = !isExprStmt(stack, i-2)

		return s
	}

	bare := plain && pointerRoot

	switch p := parent.(type) {
	case *ast.AssignStmt:
		if p.Tok != token.DEFINE && containsExpr(p.Lhs, loc) {
			if bare {
				s.kind = siteEscape
			} else if !plain {
				s.kind = siteWrite
			}

			return s
		}
		if bare {
			s.kind = siteEscape
		}
	case *ast.IncDecStmt:
		if !plain {
			s.kind = siteWrite
		}
	case *ast.RangeStmt:
		if p.Tok == token.ASSIGN && (p.Key == loc || p.Value == loc) && !plain {
			s.kind = siteWrite
		}
	case *ast.UnaryExpr:
		if p.Op != token.AND {
			return s
		}
		if bare {
			s.kind = siteEscape

			return s
		}
		s.expr = p
		call, ok := stack[i-2].(*ast.CallExpr)
		idx := argIndex(call, p)
		if !ok || idx < 0 {
			s.kind = siteEscape

			return s
		}
		s.kind = siteAddrCall
		s.callee, s.param = a.calleeParam(call, idx)
		s.call = call
		s.resultUsed = !isExprStmt(stack, i-3)
	case *ast.CallExpr:
		if !bare {
			return s
		}
		idx := argIndex(p, loc)
		if idx < 0 {
			s.kind = siteEscape

			return s
		}
		s.kind = siteArgCall
		s.callee, s.param = a.calleeParam(p, idx)
		s.call = p
	case *ast.BinaryExpr:
	default:
		if bare {
			s.kind = siteEscape
		}
	}

	return s
}

// calleeParam resolves the function called by call and the index of the
// parameter that receives argument idx. The function is nil when unknown.
func (a *analyzer) calleeParam(call *ast.CallExpr, idx int) (*types.Func, int) {
	fn, ok := typeutil.Callee(a.pass.TypesInfo, call).(*types.Func)
	if !ok {
		return nil, 0
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return nil, 0
	}

	// In a method expression call T.M(recv, args...), the first argument is
	// the receiver.
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if s := a.pass.TypesInfo.Selections[sel]; s != nil && s.Kind() == types.MethodExpr {
			idx--
			if idx < 0 {
				return nil, 0
			}
		}
	}

	n := sig.Params().Len()
	if sig.Variadic() && idx >= n-1 {
		idx = n - 1
	}
	if idx >= n {
		return nil, 0
	}

	return fn.Origin(), idx
}

// computeFacts derives the write fact of every function in the package. Facts
// depend on the facts of callees within the package, so iterate to a fixpoint;
// effects only grow, so the iteration terminates.
func (a *analyzer) computeFacts() {
	for changed := true; changed; {
		changed = false
		for _, fi := range a.funcs {
			f := writeFact{Params: make([]effect, len(fi.fact.Params))}
			for obj, sites := range fi.sites {
				e := effectNone
				for _, s := range sites {
					e = max(e, a.effectOf(s))
				}
				if i := fi.index[obj]; i < 0 {
					f.Recv = e
				} else {
					f.Params[i] = e
				}
			}
			if !fi.fact.equal(f) {
				fi.fact = f
				changed = true
			}
		}
	}

	for fn, fi := range a.funcs {
		if !fi.fact.isZero() {
			a.pass.ExportObjectFact(fn, &fi.fact)
		}
	}
}

func (a *analyzer) effectOf(s site) effect {
	switch s.kind {
	case siteWrite:
		return s.path
	case siteMethodCall:
		if s.path == effectShared {
			return effectShared
		}
		f := a.factOf(s.callee)
		if f == nil {
			return effectShared
		}

		return f.Recv
	case siteAddrCall:
		if s.path == effectShared {
			return effectShared
		}
		f := a.factOf(s.callee)
		if f == nil {
			return effectShared
		}

		return f.param(s.param)
	case siteArgCall:
		f := a.factOf(s.callee)
		if f == nil {
			return effectShared
		}

		return f.param(s.param)
	case siteEscape:
		return effectShared
	default:
		return effectNone
	}
}

// factOf returns the write fact of fn, or nil when it is unknown.
func (a *analyzer) factOf(fn *types.Func) *writeFact {
	if fn == nil {
		return nil
	}
	if fi, ok := a.funcs[fn]; ok {
		return &fi.fact
	}

	var f writeFact
	if a.pass.ImportObjectFact(fn, &f) {
		return &f
	}

	return nil
}

func (a *analyzer) checkLoops() {
	for _, li := range a.loops {
		if li.hasGoto {
			continue
		}

		var candidates, reads []site
		for _, s := range li.sites {
			if a.isDeadCandidate(s) {
				candidates = append(candidates, s)
			} else {
				reads = append(reads, s)
			}
		}

		for _, c := range candidates {
			if c.looped || c.deferred || readAfter(reads, c) {
				continue
			}
			a.report(li.v, c)
		}
	}
}

// isDeadCandidate reports whether s writes only to storage owned by the loop
// variable, without consuming anything derived from it.
func (a *analyzer) isDeadCandidate(s site) bool {
	switch s.kind {
	case siteWrite:
		return s.path == effectDirect
	case siteMethodCall, siteAddrCall:
		if a.effectOf(s) != effectDirect {
			return false
		}

		return !s.resultUsed || allResultsError(s.callee)
	default:
		return false
	}
}

func (a *analyzer) report(v *types.Var, s site) {
	name := v.Name()

	var msg string
	switch s.kind {
	case siteWrite:
		msg = fmt.Sprintf("write to %s has no effect", types.ExprString(s.expr))
	case siteMethodCall:
		msg = fmt.Sprintf("%s writes to %s, which has no effect", types.ExprString(s.expr), name)
	case siteAddrCall:
		msg = fmt.Sprintf("%s writes to %s through %s, which has no effect",
			types.ExprString(s.call.Fun), name, types.ExprString(s.expr))
	default:
		return
	}

	a.pass.Reportf(s.expr.Pos(), "%s: %s is a copy of the range element", msg, name)
}

// readAfter reports whether any read may observe the loop variable after c.
func readAfter(reads []site, c site) bool {
	for _, r := range reads {
		if r.deferred || r.expr.Pos() > c.stmt.End() {
			return true
		}
	}

	return false
}

// loopContext reports whether the identifier at the top of stack is inside a
// deferred context (defer, go, or a function literal) or a nested loop within
// the body of rs.
func loopContext(stack []ast.Node, rs *ast.RangeStmt) (deferred, looped bool) {
	inside := false
	for _, n := range stack {
		if n == rs {
			inside = true

			continue
		}
		if !inside {
			continue
		}
		switch n.(type) {
		case *ast.DeferStmt, *ast.GoStmt, *ast.FuncLit:
			deferred = true
		case *ast.ForStmt, *ast.RangeStmt:
			looped = true
		}
	}

	return deferred, looped
}

// selectionDerefs counts the pointer indirections on the path from the
// receiver of sel to the selected field or method. ok is false when the path
// goes through something other than structs.
func selectionDerefs(sel *types.Selection) (derefs int, ok bool) {
	t := sel.Recv()
	if p, isPtr := under(t).(*types.Pointer); isPtr {
		derefs++
		t = p.Elem()
	}

	index := sel.Index()
	for _, i := range index[:len(index)-1] {
		st, isStruct := under(t).(*types.Struct)
		if !isStruct {
			return derefs, false
		}
		t = st.Field(i).Type()
		if p, isPtr := under(t).(*types.Pointer); isPtr {
			derefs++
			t = p.Elem()
		}
	}

	return derefs, true
}

func hasPointerReceiver(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}

	return isPointer(sig.Recv().Type())
}

func allResultsError(fn *types.Func) bool {
	if fn == nil {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Results().Len() == 0 {
		return false
	}

	errType := types.Universe.Lookup("error").Type()
	for i := range sig.Results().Len() {
		if !types.Identical(sig.Results().At(i).Type(), errType) {
			return false
		}
	}

	return true
}

func hasGoto(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if b, ok := n.(*ast.BranchStmt); ok && b.Tok == token.GOTO {
			found = true
		}

		return !found
	})

	return found
}

func enclosingStmt(stack []ast.Node) ast.Stmt {
	for i := len(stack) - 1; i >= 0; i-- {
		if s, ok := stack[i].(ast.Stmt); ok {
			return s
		}
	}

	return nil
}

func isExprStmt(stack []ast.Node, i int) bool {
	if i < 0 {
		return false
	}
	_, ok := stack[i].(*ast.ExprStmt)

	return ok
}

func argIndex(call *ast.CallExpr, arg ast.Expr) int {
	if call == nil {
		return -1
	}
	for i, a := range call.Args {
		if a == arg {
			return i
		}
	}

	return -1
}

func containsExpr(list []ast.Expr, e ast.Expr) bool {
	for _, x := range list {
		if x == e {
			return true
		}
	}

	return false
}

func idents(exprs ...ast.Expr) []*ast.Ident {
	var out []*ast.Ident
	for _, e := range exprs {
		if id, ok := e.(*ast.Ident); ok && id.Name != "_" {
			out = append(out, id)
		}
	}

	return out
}

func under(t types.Type) types.Type {
	if t == nil {
		return nil
	}

	return types.Unalias(t).Underlying()
}

func isPointer(t types.Type) bool {
	_, ok := under(t).(*types.Pointer)

	return ok
}

// isCopyType reports whether assigning a value of type t copies its storage
// in a way that makes writes to the copy invisible to the original.
func isCopyType(t types.Type) bool {
	switch under(t).(type) {
	case *types.Struct, *types.Array:
		return true
	}

	return false
}
