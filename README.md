# deadmut

A Go linter that detects mutations to range loop value copies.

Name origin: "dead mutation" = mutation with no effect

## Background

Go's range loop copies values, so modifications to the copy don't affect the original slice:

```go
for _, user := range users {
    user.Name = "updated"  // Does NOT update the original slice!
}
```

This is:
- A common pitfall for beginners
- Not a compile error
- Not a runtime panic
- A silent bug

## Comparison with Existing Linters

| Linter             | Detects                            | Range loop value mutation |
|--------------------|------------------------------------|---------------------------|
| staticcheck SA4005 | Field assignment to value receiver | No                        |
| copyloopvar        | `i := i` copy pattern              | Different issue           |
| ineffassign        | Assignment to unused variable      | No                        |
| loopclosure        | Capture in goroutine/defer         | Different issue           |
| **deadmut**        | **Mutation to range loop value**   | **Yes**                   |

## Installation

```bash
go install github.com/mickamy/deadmut@latest
```

## Usage

```bash
deadmut ./...
```

## What It Detects

### 1. Field Assignment

```go
for _, user := range users {
    user.Name = "updated"  // deadmut: mutation to range value copy has no effect
}
```

### 2. Pointer Receiver Method Call

```go
for _, user := range users {
    user.SetName("updated")  // deadmut: pointer receiver method on range value copy has no effect
}
```

### 3. Taking Pointer of Value

```go
for _, user := range users {
    updateUser(&user)  // deadmut: pointer to range value copy may not have intended effect
}
```

## What It Ignores

### Index Access

```go
for i := range users {
    users[i].Name = "updated"  // OK - direct modification
}
```

### Slice of Pointers

```go
for _, user := range users {  // users is []*User
    user.Name = "updated"  // OK - modifying via pointer
}
```

### Value Used After Mutation

```go
for _, user := range users {
    user.Name = "updated"
    process(user)  // OK - intentionally using the copy
}
```

### Collecting Copies

```go
for _, user := range users {
    user.Name = "updated"
    results = append(results, user)  // OK - collecting modified copies
}
```

## How to Fix

```go
// Before (bug)
for _, user := range users {
    user.Name = "updated"
}

// After (fix 1: use index)
for i := range users {
    users[i].Name = "updated"
}

// After (fix 2: use pointer slice)
for _, user := range users {  // users is []*User
    user.Name = "updated"
}
```

## License

[MIT](./LICENSE)
