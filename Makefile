.PHONY: all generate lint test bench update deps

all: generate lint test

## generate: regenerate API code from the Telegram Bot API documentation
generate:
	go generate ./...

## lint: vet and golangci-lint
lint:
	go vet ./...
	golangci-lint run ./...

## test: race tests with coverage
test:
	go test -race -failfast -cover ./...

## bench: run benchmarks with allocation stats
bench:
	go test -run '^$$' -bench . -benchmem ./...

## update: bump dependencies (explicit, never part of `all`)
update:
	go get -u ./... && go mod tidy

## deps: install golangci-lint
deps:
	curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $$(go env GOPATH)/bin
