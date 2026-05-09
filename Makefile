.PHONY: backend frontend dev swagger lint build

## Start Go backend (requires backend/.env)
backend:
	cd backend && go run main.go

## Start Next.js frontend dev server
frontend:
	cd frontend && npm run dev

## Install frontend dependencies
install:
	cd frontend && npm install

## Run both (requires two terminals — use this as reference)
dev:
	@echo "Run in separate terminals:"
	@echo "  make backend"
	@echo "  make frontend"

## Build production Go binary
build:
	cd backend && go build -o bin/sentinelbag .

## Run Go tests
test:
	cd backend && go test ./...

## Format and vet
lint:
	cd backend && gofmt -w . && go vet ./...

## Generate Swagger docs from annotations (requires swag CLI)
## Install: go install github.com/swaggo/swag/cmd/swag@latest
## Then add to main.go: import _ "sentinelbag/docs" + httpSwagger route
swagger:
	cd backend && swag init -g main.go -o docs --parseDependency

## Clean build artifacts
clean:
	rm -f backend/bin/sentinelbag
	rm -rf backend/docs/
