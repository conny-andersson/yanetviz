.PHONY: setup web build run test test-browser format

setup:
	cd web && npm ci
	go mod download

web:
	cd web && npm run build

build: web
	go build -buildvcs=false -o yanetviz ./cmd/yanetviz

run: build
	./yanetviz

test: web
	go test -race ./...
	go vet ./...
	cd web && npm run check:format

test-browser: web
	cd web && npm run test:e2e

format:
	gofmt -w assets.go cmd internal
	cd web && npm run format
