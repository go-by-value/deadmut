package a

import (
	"encoding/json"
	"fmt"

	"b"
)

type Address struct {
	City string
}

type Meta struct {
	Note string
}

type User struct {
	Name string
	Age  int
	Tags []string
	Meta *Meta
	Addr Address
	Arr  [2]int
}

func (u *User) SetName(name string) { // want SetName:"recv=direct"
	u.Name = name
}

func (u *User) Summary() string {
	return fmt.Sprintf("%s (%d)", u.Name, u.Age)
}

func (u *User) AddTag(tag string) { // want AddTag:"recv=direct"
	u.Tags = append(u.Tags, tag)
}

func (u *User) ClearFirstTag() { // want ClearFirstTag:"recv=shared"
	u.Tags[0] = ""
}

func (u *User) SetNote(note string) { // want SetNote:"recv=shared"
	u.Meta.Note = note
}

func (u *User) Rename(name string) { // want Rename:"recv=direct"
	u.SetName(name)
}

func (u *User) Normalize() error { // want Normalize:"recv=direct"
	if u.Name == "" {
		u.Name = "anonymous"
	}

	return nil
}

func (u *User) Birthday() int { // want Birthday:"recv=direct"
	u.Age++

	return u.Age
}

func (u User) WithName(name string) User {
	u.Name = name

	return u
}

func update(u *User) { // want update:"recv=none p0=direct"
	u.Name = "updated"
}

func inspect(u *User) int {
	return u.Age
}

func keep(u *User) { // want keep:"recv=none p0=shared"
	registry = append(registry, u)
}

func viaInterface(v any) {
	_ = v
}

var registry []*User

// --- Reported ---

func fieldAssignment(users []User) {
	for _, u := range users {
		u.Name = "updated" // want `write to u.Name has no effect: u is a copy of the range element`
	}
}

func incDec(users []User) {
	for _, u := range users {
		u.Age++ // want `write to u.Age has no effect`
	}
}

func nestedStruct(users []User) {
	for _, u := range users {
		u.Addr.City = "Tokyo" // want `write to u.Addr.City has no effect`
	}
}

func arrayField(users []User) {
	for _, u := range users {
		u.Arr[0] = 1 // want `write to u.Arr\[0\] has no effect`
	}
}

func sliceHeaderField(users []User) {
	for _, u := range users {
		u.Tags = nil // want `write to u.Tags has no effect`
	}
}

func multipleWrites(users []User) {
	for _, u := range users {
		u.Name = "a" // want `write to u.Name has no effect`
		u.Age = 1    // want `write to u.Age has no effect`
	}
}

func writeAfterRead(users []User) {
	for _, u := range users {
		fmt.Println(u.Name)
		u.Name = "updated" // want `write to u.Name has no effect`
	}
}

func pointerReceiverMethod(users []User) {
	for _, u := range users {
		u.SetName("updated") // want `u.SetName writes to u, which has no effect`
	}
}

func pointerReceiverViaCall(users []User) {
	for _, u := range users {
		u.Rename("updated") // want `u.Rename writes to u, which has no effect`
	}
}

func appendingMethod(users []User) {
	for _, u := range users {
		u.AddTag("x") // want `u.AddTag writes to u, which has no effect`
	}
}

func errorOnlyResult(users []User) {
	for _, u := range users {
		if err := u.Normalize(); err != nil { // want `u.Normalize writes to u, which has no effect`
			return
		}
	}
}

func addressToFunc(users []User) {
	for _, u := range users {
		update(&u) // want `update writes to u through &u, which has no effect`
	}
}

func addressToFuncErrorResult(users []b.Counter) {
	for _, c := range users {
		if err := b.Update(&c, 1); err != nil { // want `b.Update writes to c through &c, which has no effect`
			return
		}
	}
}

func addressOfField(users []User) {
	for _, u := range users {
		setCity(&u.Addr) // want `setCity writes to u through &u.Addr, which has no effect`
	}
}

func setCity(a *Address) { // want setCity:"recv=none p0=direct"
	a.City = "Tokyo"
}

func crossPackageMethod(counters []b.Counter) {
	for _, c := range counters {
		c.Inc() // want `c.Inc writes to c, which has no effect`
	}
}

func crossPackageFunc(counters []b.Counter) {
	for _, c := range counters {
		b.Reset(&c) // want `b.Reset writes to c through &c, which has no effect`
	}
}

func mapValue(users map[string]User) {
	for _, u := range users {
		u.Name = "updated" // want `write to u.Name has no effect`
	}
}

func arrayElement(users [3]User) {
	for _, u := range users {
		u.Name = "updated" // want `write to u.Name has no effect`
	}
}

func arrayPointer(users *[3]User) {
	for _, u := range users {
		u.Name = "updated" // want `write to u.Name has no effect`
	}
}

func channelValue(ch chan User) {
	for u := range ch {
		u.Name = "updated" // want `write to u.Name has no effect`
	}
}

func iteratorValue(seq func(yield func(int, User) bool)) {
	for _, u := range seq {
		u.Name = "updated" // want `write to u.Name has no effect`
	}
}

func arrayCopy(rows [][2]int) {
	for _, row := range rows {
		row[0] = 1 // want `write to row\[0\] has no effect`
	}
}

func inBranch(users []User) {
	for _, u := range users {
		if u.Age > 0 {
			u.Name = "adult" // want `write to u.Name has no effect`
		}
	}
}

func writeThenIndexedRead(users []User) {
	for i, u := range users {
		u.Name = "updated" // want `write to u.Name has no effect`
		fmt.Println(users[i].Name)
	}
}

// --- Not reported ---

func indexAccess(users []User) {
	for i := range users {
		users[i].Name = "updated"
	}
}

func pointerSlice(users []*User) {
	for _, u := range users {
		u.Name = "updated"
	}
}

func readAfterWrite(users []User) {
	for _, u := range users {
		u.Name = "updated"
		fmt.Println(u)
	}
}

func collected(users []User) []User {
	var out []User
	for _, u := range users {
		u.Name = "updated"
		out = append(out, u)
	}

	return out
}

func readFieldAfterWrite(users []User) {
	for _, u := range users {
		u.Age = 1
		fmt.Println(u.Name)
	}
}

func throughSlice(users []User) {
	for _, u := range users {
		u.Tags[0] = "updated"
	}
}

func throughPointerField(users []User) {
	for _, u := range users {
		u.Meta.Note = "updated"
	}
}

func readOnlyMethod(users []User) {
	for _, u := range users {
		fmt.Println(u.Summary())
	}
}

func valueReceiverMethod(users []User) {
	for _, u := range users {
		fmt.Println(u.WithName("x"))
	}
}

func sharedWriteMethod(users []User) {
	for _, u := range users {
		u.ClearFirstTag()
		u.SetNote("x")
	}
}

func resultConsumed(users []User) {
	for _, u := range users {
		fmt.Println(u.Birthday())
	}
}

func readOnlyFunc(users []User) {
	for _, u := range users {
		fmt.Println(inspect(&u))
	}
}

func escapingAddress(users []User) {
	for _, u := range users {
		keep(&u)
	}
}

func addressStored(users []User) {
	for _, u := range users {
		p := &u
		_ = p
	}
}

func unknownCallee(data [][]byte, users []User) {
	for i, u := range users {
		_ = json.Unmarshal(data[i], &u)
	}
}

func interfaceParam(users []User) {
	for _, u := range users {
		viaInterface(&u)
	}
}

func deferredRead(users []User) {
	for _, u := range users {
		u.Name = "updated"
		defer fmt.Println(u.Name)
	}
}

func closureRead(users []User) {
	for _, u := range users {
		u.Name = "updated"
		f := func() { fmt.Println(u.Name) }
		f()
	}
}

func writeInClosure(users []User) {
	for _, u := range users {
		f := func() { u.Name = "updated" }
		f()
	}
}

func nestedLoop(users []User) {
	for _, u := range users {
		for range 2 {
			fmt.Println(u.Name)
			u.Name = "updated"
		}
	}
}

func withGoto(users []User) {
	for _, u := range users {
	again:
		if u.Age < 0 {
			u.Age = 0
			goto again
		}
	}
}

func shadowed(users []User) {
	for _, u := range users {
		u := u
		u.Name = "updated"
	}
}

func crossPackageReadOnly(counters []b.Counter) {
	for _, c := range counters {
		fmt.Println(c.Get(), b.Inspect(&c))
	}
}

func crossPackageShared(counters []b.Counter) {
	for _, c := range counters {
		c.Touch()
		b.Store(&c)
	}
}

func crossPackageResultConsumed(counters []b.Counter) {
	for _, c := range counters {
		fmt.Println(c.Next())
	}
}

func assignedLoopVar(users []User) {
	var u User
	for _, u = range users {
		u.Name = "updated"
	}
	_ = u
}

func wholeReassign(users []User) {
	for _, u := range users {
		u = User{}
		_ = u
	}
}

type Wrapper struct {
	*User
}

func embeddedPointer(ws []Wrapper) {
	for _, w := range ws {
		w.Name = "updated"
		w.SetName("updated")
	}
}

type Holder struct {
	User
}

func embeddedValue(hs []Holder) {
	for _, h := range hs {
		h.Name = "updated" // want `write to h.Name has no effect`
		h.SetName("x")     // want `h.SetName writes to h, which has no effect`
	}
}
