package a

type User struct {
	Name string
	Age  int
}

func (u *User) SetName(name string) {
	u.Name = name
}

func (u User) GetName() string {
	return u.Name
}

// Bad: mutation to range value copy
func badFieldAssignment() {
	users := []User{{Name: "alice"}, {Name: "bob"}}
	for _, user := range users {
		user.Name = "updated" // want "mutation to range value copy has no effect"
	}
}

// Bad: pointer receiver method on range value copy
func badPointerReceiverMethod() {
	users := []User{{Name: "alice"}, {Name: "bob"}}
	for _, user := range users {
		user.SetName("updated") // want "pointer receiver method on range value copy has no effect"
	}
}

// Bad: taking pointer of range value copy
func badTakingPointer() {
	users := []User{{Name: "alice"}, {Name: "bob"}}
	for _, user := range users {
		updateUser(&user) // want "pointer to range value copy may not have intended effect"
	}
}

func updateUser(u *User) {
	u.Name = "updated"
}

// Good: using index
func goodIndexAccess() {
	users := []User{{Name: "alice"}, {Name: "bob"}}
	for i := range users {
		users[i].Name = "updated"
	}
}

// Good: slice of pointers
func goodPointerSlice() {
	users := []*User{{Name: "alice"}, {Name: "bob"}}
	for _, user := range users {
		user.Name = "updated"
	}
}

// Good: value used after mutation
func goodValueUsedAfter() {
	users := []User{{Name: "alice"}, {Name: "bob"}}
	var results []User
	for _, user := range users {
		user.Name = "updated"
		results = append(results, user)
	}
}

// Good: value receiver method (no mutation)
func goodValueReceiverMethod() {
	users := []User{{Name: "alice"}, {Name: "bob"}}
	for _, user := range users {
		_ = user.GetName()
	}
}

// Good: copy is intentionally used
func goodCopyProcess() {
	users := []User{{Name: "alice"}, {Name: "bob"}}
	for _, user := range users {
		user.Name = "updated"
		process(user)
	}
}

func process(u User) {}

// === Map cases ===

// Bad: mutation to map value copy
func badMapFieldAssignment() {
	users := map[string]User{"alice": {Name: "alice"}, "bob": {Name: "bob"}}
	for _, user := range users {
		user.Name = "updated" // want "mutation to range value copy has no effect"
	}
}

// Bad: pointer receiver method on map value copy
func badMapPointerReceiverMethod() {
	users := map[string]User{"alice": {Name: "alice"}}
	for _, user := range users {
		user.SetName("updated") // want "pointer receiver method on range value copy has no effect"
	}
}

// Bad: taking pointer of map value copy
func badMapTakingPointer() {
	users := map[string]User{"alice": {Name: "alice"}}
	for _, user := range users {
		updateUser(&user) // want "pointer to range value copy may not have intended effect"
	}
}

// Good: map of pointers
func goodMapPointerValue() {
	users := map[string]*User{"alice": {Name: "alice"}}
	for _, user := range users {
		user.Name = "updated"
	}
}

// Good: map value used after mutation
func goodMapValueUsedAfter() {
	users := map[string]User{"alice": {Name: "alice"}}
	var results []User
	for _, user := range users {
		user.Name = "updated"
		results = append(results, user)
	}
}
