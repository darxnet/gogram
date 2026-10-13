.PHONY: all generate lint test bench update deps

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

## bench: run benchmarks with allocation stats
bench:
	go test -run '^$$' -bench . -benchmem ./...

## update: bump dependencies (explicit, never part of `all`)
update:
	go get -u ./... && go mod tidy

## deps: install golangci-lint (the script verifies the archive checksum)
deps:
	curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $$(go env GOPATH)/bin
