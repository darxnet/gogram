.PHONY: all generate lint test

all: generate lint test

## generate: regenerate API code from the Telegram Bot API documentation
generate:
	go generate ./...

## lint: vet and golangci-lint
lint:
	go vet ./... && go vet -C cmd/gen ./...
	golangci-lint run ./... && cd cmd/gen && golangci-lint run ./...

## test: race tests with coverage, plus the SQLite integration tests of sqlstore
test:
	go test -race -failfast -cover ./...
	cd sqlstore/sqlitetest && go test -race ./...
