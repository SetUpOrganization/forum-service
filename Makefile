SERVICE=backend

build:
	go build -o bin/${SERVICE} ./cmd/main/main.go

tests:
	go test ./internal/... -timeout 1m -v