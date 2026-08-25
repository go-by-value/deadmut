.PHONY: build install test lint vet clean

build:
	go build -trimpath -ldflags "-s -w" -o bin/deadmut ./cmd/deadmut

install:
	go install ./cmd/deadmut

test:
	go test -race ./...

lint:
	golangci-lint run

# Run deadmut on its own source.
vet: build
	go vet -vettool=bin/deadmut ./...

clean:
	rm -rf bin
