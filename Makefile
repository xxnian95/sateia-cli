.PHONY: build test check

build:
	go build -trimpath -o bin/sateia ./cmd/sateia

test:
	go test ./...

check:
	go vet ./...
	go test ./...
