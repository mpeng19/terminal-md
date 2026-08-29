BIN := tmd

.PHONY: build install uninstall run fmt vet clean

build:
	go build -o $(BIN) ./cmd/tmd

install:
	go install ./cmd/tmd

uninstall:
	rm -f "$$(go env GOPATH)/bin/$(BIN)"

run: build
	./$(BIN) examples/demo.md

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -f $(BIN)
