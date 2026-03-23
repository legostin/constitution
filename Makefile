.PHONY: build test lint clean install

# Build both binaries
build:
	go build -o bin/constitution ./cmd/constitution
	go build -o bin/constitutiond ./cmd/constitutiond

# Run all tests
test:
	go test ./... -count=1 -race

# Run tests with verbose output
test-v:
	go test ./... -count=1 -race -v

# Run go vet
lint:
	go vet ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Install binaries to $GOPATH/bin
install:
	go install ./cmd/constitution
	go install ./cmd/constitutiond

# Quick smoke test: pipe a PreToolUse event through the binary
smoke-test: build
	echo '{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"rm -rf /"},"cwd":"'$(PWD)'"}' | \
		CONSTITUTION_CONFIG=configs/constitution.yaml ./bin/constitution; \
		echo "Exit code: $$?"

# Run the remote service locally
run-server: build
	./bin/constitutiond --config configs/constitution.yaml --addr :8081

# Format code
fmt:
	gofmt -s -w .

# Tidy modules
tidy:
	go mod tidy
