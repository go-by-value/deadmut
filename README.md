# deadmut

[![CI](https://github.com/go-by-value/deadmut/actions/workflows/ci.yaml/badge.svg)](https://github.com/go-by-value/deadmut/actions/workflows/ci.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-by-value/deadmut.svg)](https://pkg.go.dev/github.com/go-by-value/deadmut)
[![Release](https://img.shields.io/github/v/release/go-by-value/deadmut)](https://github.com/go-by-value/deadmut/releases/latest)

deadmut is a Go analyzer that reports mutations of range loop value copies that have no effect.

```go
for _, user := range users {
	user.Name = strings.TrimSpace(user.Name) // deadmut: write to user.Name has no effect: user is a copy of the range element
}
```

The value variable of a range loop is a copy of the element. Writing to it changes the copy, not the slice, and Go
compiles it without complaint. deadmut reports such writes when nothing reads the copy afterwards, which is when the
write is a bug rather than a scratch value.

The name is short for "dead mutation."

## What it reports

deadmut looks at range loops whose value variable is a struct or an array, taken from a slice, an array, a map, a
channel, or an iterator function. It reports three forms of mutation.

Direct writes to the copy, including nested structs and array elements:

```go
for _, user := range users {
	user.Name = "updated" // reported
	user.Address.City = "NY" // reported
	user.Visits++            // reported
}
```

Calls to pointer-receiver methods that write to the receiver:

```go
for _, user := range users {
	user.SetName("updated") // reported: SetName writes to user.Name
}
```

Passing the address of the copy to a function that writes through it:

```go
for _, user := range users {
	if err := normalize(&user); err != nil { // reported: normalize writes to *user
		return err
	}
}
```

To know whether a method or a function writes through its pointer receiver or pointer parameters, deadmut analyzes their
bodies and exports the result as a fact. Facts cross package boundaries, so `bytes.Buffer.WriteString` is known to write
and `bytes.Buffer.Len` is known not to.

## What it does not report

deadmut is designed to stay quiet unless the mutation is certainly lost. It does not report when:

- The copy is read afterwards, including in a `defer`, a `go` statement, or a closure.

  ```go
  for _, user := range users {
      user.Name = "updated"
      out = append(out, user) // the copy is the point
  }
  ```

- The write goes through a pointer, slice, or map reachable from the copy. Those are shared with the original element,
  so the write is observable.

  ```go
  for _, user := range users {
      user.Tags[0] = "updated"   // shared backing array
      user.Profile.Bio = "..."   // Profile is a pointer
  }
  ```

- The pointer-receiver method or the function only reads, or its behavior cannot be determined (function values,
  interface methods, interface-typed parameters such as `json.Unmarshal`, or code that uses `unsafe`).

- The call consumes a result other than an `error`. A call like `n := user.Next()` uses the copy for its return value.

- The mutation happens inside a loop nested in the range body, inside a closure, or in a body that contains `goto`.

- The loop assigns to an existing variable (`for _, user = range users`), because the variable outlives the loop.

## Installation

```sh
go install github.com/go-by-value/deadmut/cmd/deadmut@latest
```

Prebuilt binaries are available on the [releases page](https://github.com/go-by-value/deadmut/releases).

## Usage

```sh
deadmut ./...
```

deadmut is a standard `go/analysis` analyzer, so it also works as a `go vet` tool:

```sh
go vet -vettool=$(which deadmut) ./...
```

### golangci-lint

deadmut ships a [module plugin](https://golangci-lint.run/docs/plugins/module-plugins/). Add it to `.custom-gcl.yml`:

```yaml
version: v2.13.0
plugins:
  - module: github.com/go-by-value/deadmut
    import: github.com/go-by-value/deadmut/plugin
    version: v0.1.0
```

Build the custom binary with `golangci-lint custom` and enable the linter in `.golangci.yaml`:

```yaml
version: "2"
linters:
  enable:
    - deadmut
  settings:
    custom:
      deadmut:
        type: module
        description: Reports mutations of range loop value copies that have no effect.
```

## Relation to `unusedwrite`

The `unusedwrite` analyzer in `go vet` (available in golangci-lint through `govet`) also reports the first form, direct
field writes to a copy, using SSA. deadmut differs in scope:

|                                                      | `unusedwrite` | deadmut                                             |
|------------------------------------------------------|---------------|-----------------------------------------------------|
| Direct field writes to a range copy                  | Yes           | Yes                                                 |
| Pointer-receiver method calls on a range copy        | No            | Yes, when the method writes to the receiver         |
| Passing `&copy` to a function that writes through it | No            | Yes, when the function writes through the parameter |
| Writes to value receivers and other local copies     | Yes           | No, range loops only                                |

The two are complementary. Running both is fine: they agree on the overlapping cases.

## Limitations

- deadmut treats any read that appears later in the loop body as a read after the mutation, even when control flow makes
  it unreachable. This trades some misses for no false positives.
- Writes through interface-typed parameters are not tracked, so `json.Unmarshal(data, &user)` is not reported even when
  `user` is never read.
- Facts are only available for packages compiled from source. Methods from packages without source are treated as
  unknown and are not reported.

## Development

```sh
make test  # go test -race ./...
make lint  # golangci-lint run
make vet   # run deadmut on its own source
```

## License

[MIT](./LICENSE)
