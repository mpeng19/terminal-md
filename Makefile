BIN := tmd
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build install uninstall run fmt vet test clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/tmd

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/tmd

uninstall:
	rm -f "$$(go env GOPATH)/bin/$(BIN)"

run: build
	./$(BIN) examples/demo.md

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -f $(BIN)
