BINARY_NAME=tow
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
BUILD_DATE=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE)"

.PHONY: build clean test lint fmt vet cover install dist

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/tow

install: build
	cp bin/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME) 2>/dev/null || \
	cp bin/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)

clean:
	rm -rf bin/ dist/ coverage.out .tow/

test:
	go test ./... -v -count=1

test-race:
	go test ./... -v -race -count=1

cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out
	@echo ""
	@echo "To view HTML report: go tool cover -html=coverage.out"

fmt:
	gofmt -w cmd/ internal/ integrations/
	goimports -w cmd/ internal/ integrations/ 2>/dev/null || true

fmt-check:
	@test -z "$$(gofmt -l cmd/ internal/ integrations/)" || { echo "gofmt needed:"; gofmt -l cmd/ internal/ integrations/; exit 1; }

vet:
	go vet ./...

lint:
	golangci-lint run ./...

check: fmt-check vet test
	@echo "All checks passed"

# Cross-compilation
dist: dist-linux dist-darwin

dist-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 ./cmd/tow
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 ./cmd/tow

dist-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/tow
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/tow
