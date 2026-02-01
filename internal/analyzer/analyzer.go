// Package analyzer provides a linter that detects mutations to range loop value copies.
package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer detects mutations to range loop value copies that have no effect.
var Analyzer = &analysis.Analyzer{
	Name:     "deadmut",
	Doc:      "detects mutations to range loop value copies that have no effect",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	ispct, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, nil
	}

	nodeFilter := []ast.Node{
		(*ast.RangeStmt)(nil),
	}

	ispct.Preorder(nodeFilter, func(n ast.Node) {
		rangeStmt, ok := n.(*ast.RangeStmt)
		if !ok {
			return
		}
		checkRangeStmt(pass, rangeStmt)
	})

	return nil, nil
}

func checkRangeStmt(pass *analysis.Pass, rangeStmt *ast.RangeStmt) {
	// Get the value variable (the second variable in "for _, v := range ...")
	valueIdent, ok := rangeStmt.Value.(*ast.Ident)
	if !ok || valueIdent.Name == "_" {
		return
	}

	// Check if the range target is a slice/array of non-pointer structs
	rangeType := pass.TypesInfo.TypeOf(rangeStmt.X)
	if rangeType == nil {
		return
	}

	elemType := getElementType(rangeType)
	if elemType == nil {
		return
	}

	// If the element type is a pointer, mutations are effective
	if isPointerType(elemType) {
		return
	}

	// If the element type is not a struct, skip
	if !isStructType(elemType) {
		return
	}

	// Track usages of the value variable in the loop body
	mutations := findMutations(pass, rangeStmt.Body, valueIdent)

	for _, mut := range mutations {
		if !isValueUsedAfterMutation(rangeStmt.Body, valueIdent, mut.end) {
			pass.Reportf(mut.pos, "%s", mut.message)
		}
	}
}

type mutation struct {
	pos     token.Pos
	end     token.Pos
	message string
}

func findMutations(pass *analysis.Pass, body *ast.BlockStmt, valueIdent *ast.Ident) []mutation {
	var mutations []mutation

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// Check for field assignments: v.Field = ...
			for _, lhs := range node.Lhs {
				if sel, ok := lhs.(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok {
						if ident.Name == valueIdent.Name && isSameObject(pass, ident, valueIdent) {
							mutations = append(mutations, mutation{
								pos:     node.Pos(),
								end:     node.End(),
								message: "mutation to range value copy has no effect",
							})
						}
					}
				}
			}

		case *ast.CallExpr:
			// Check for pointer receiver method calls: v.Method()
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if ident.Name == valueIdent.Name && isSameObject(pass, ident, valueIdent) {
						if isPointerReceiverMethod(pass, node) {
							mutations = append(mutations, mutation{
								pos:     node.Pos(),
								end:     node.End(),
								message: "pointer receiver method on range value copy has no effect",
							})
						}
					}
				}
			}

			// Check for taking pointer of value: someFunc(&v)
			for _, arg := range node.Args {
				if unary, ok := arg.(*ast.UnaryExpr); ok && unary.Op == token.AND {
					if ident, ok := unary.X.(*ast.Ident); ok {
						if ident.Name == valueIdent.Name && isSameObject(pass, ident, valueIdent) {
							mutations = append(mutations, mutation{
								pos:     unary.Pos(),
								end:     node.End(),
								message: "pointer to range value copy may not have intended effect",
							})
						}
					}
				}
			}
		}

		return true
	})

	return mutations
}

func isSameObject(pass *analysis.Pass, a, b *ast.Ident) bool {
	objA := pass.TypesInfo.ObjectOf(a)
	objB := pass.TypesInfo.ObjectOf(b)

	return objA != nil && objB != nil && objA == objB
}

func getElementType(t types.Type) types.Type {
	switch typ := t.Underlying().(type) {
	case *types.Slice:
		return typ.Elem()
	case *types.Array:
		return typ.Elem()
	case *types.Map:
		return typ.Elem()
	default:
		return nil
	}
}

func isPointerType(t types.Type) bool {
	_, ok := t.Underlying().(*types.Pointer)

	return ok
}

func isStructType(t types.Type) bool {
	_, ok := t.Underlying().(*types.Struct)

	return ok
}

func isPointerReceiverMethod(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	selection := pass.TypesInfo.Selections[sel]
	if selection == nil {
		return false
	}

	fn, ok := selection.Obj().(*types.Func)
	if !ok {
		return false
	}

	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}

	recv := sig.Recv()
	if recv == nil {
		return false
	}

	_, isPtr := recv.Type().(*types.Pointer)

	return isPtr
}

func isValueUsedAfterMutation(body *ast.BlockStmt, valueIdent *ast.Ident, afterPos token.Pos) bool {
	used := false

	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		// Only consider nodes after the mutation statement ends
		if n.Pos() <= afterPos {
			return true
		}

		if ident, ok := n.(*ast.Ident); ok {
			if ident.Name == valueIdent.Name {
				used = true

				return false
			}
		}

		return true
	})

	return used
}
