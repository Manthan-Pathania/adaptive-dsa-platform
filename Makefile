.PHONY: all build test test-coverage lint format sqlc migrate-up migrate-down migrate-status docker-up docker-down run

all: lint test build

# Build application binaries
build:
	@echo "Building server binary..."
	go build -trimpath -ldflags="-s -w" -o bin/server.exe ./cmd/server
	@echo "Building migrate binary..."
	go build -trimpath -ldflags="-s -w" -o bin/migrate.exe ./cmd/migrate
	@echo "Building admin binary..."
	go build -trimpath -ldflags="-s -w" -o bin/admin.exe ./cmd/admin
	@echo "Building import-questions binary..."
	go build -trimpath -ldflags="-s -w" -o bin/import-questions.exe ./cmd/import-questions

# Run unit tests
test:
	go test -v -count=1 ./...

# Run unit tests with race detection (requires CGO on Windows/Linux)
test-race:
	go test -v -race ./...

# Run tests with HTML coverage report
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

# Run static analysis and formatting checks
lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || (echo "Unformatted files exist. Run 'make format'" && exit 1)

# Format all Go source files
format:
	gofmt -s -w .

# Generate type-safe query code via sqlc
sqlc:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate

# Database migrations
migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

migrate-status:
	go run ./cmd/migrate status

# Local Docker services
docker-up:
	docker compose up -d

docker-down:
	docker compose down

# Run server locally (reads from .env if present)
run:
	go run ./cmd/server
