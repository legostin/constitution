.PHONY: build test lint clean install

VERSION ?= dev

# Build both binaries
build:
	go build -ldflags="-X main.version=$(VERSION)" -o bin/constitution ./cmd/constitution
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

# Install globally
install:
	go install -ldflags="-X main.version=$(VERSION)" ./cmd/constitution
	go install ./cmd/constitutiond

# Quick smoke test
smoke-test: build
	echo '{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"rm -rf /"},"cwd":"'$(PWD)'"}' | \
		CONSTITUTION_CONFIG=configs/constitution.yaml ./bin/constitution; \
		echo "Exit code: $$?"

# Run the remote service locally
run-server: build
	./bin/constitutiond --config configs/constitution.yaml --addr :8081

# Docker build
docker-build:
	docker build -t constitutiond .

# Docker run with local rules file
docker-run:
	docker compose up -d

# Format code
fmt:
	gofmt -s -w .

# Tidy modules
tidy:
	go mod tidy
