# Contributing

`gogram` is maintained by one person; issues and pull requests are welcome.
Please open an issue first for anything bigger than a bug fix, so the design is
agreed before the code is written.

## Commit messages

[Conventional Commits](https://www.conventionalcommits.org) are required: the
release version is computed from them by [`svu`](https://github.com/caarlos0/svu)
in CI when a commit lands on `main`.

| Prefix | Release |
|---|---|
| `fix:` | patch |
| `feat:` | minor |
| `feat!:` or a `BREAKING CHANGE:` footer | minor while the version is v0 (see below) |
| `chore:`, `docs:`, `test:`, `ci:`, `refactor:`, `perf:` | none |

`gogram` is v0: breaking changes stay inside v0 (`svu next --no-increment-v0`)
and v1 is a deliberate release. Mark them as `feat!:` anyway, so the release
notes show them.

A `perf:` change that users should know about belongs in a `feat:` commit.

## Generated code

The Bot API bindings (`*.gen.go`) are generated from
<https://core.telegram.org/bots/api> by `cmd/gen`. Do not edit them by hand;
change the generator or its templates and run:

    go generate ./...

A scheduled workflow regenerates them daily and opens a pull request when the
documentation changed; `make generate` does the same locally.

## Before a pull request

    go mod tidy
    go vet ./...
    go test -race -count=1 ./...
    golangci-lint run ./...                # .golangci.yml, `default: all`

    go test -C sqlstore/sqlitetest -race -count=1 ./...   # SQLite integration tests
    (cd cmd/gen && golangci-lint run ./...)                # the generator module

CI runs the same checks on the stable and oldstable Go releases, on Linux
(amd64) and macOS (arm64), plus `govulncheck`, CodeQL and a coverage gate.
Coverage is measured on hand-written code only; generated files are excluded.

## Changes on a hot path

Include `benchstat` output in the pull request description:

    go test -run=NONE -bench=. -benchmem -count=6 ./... > old.txt   # on main
    go test -run=NONE -bench=. -benchmem -count=6 ./... > new.txt   # on your branch
    benchstat old.txt new.txt

A change that makes a benchmark slower needs a reason in the commit message.

## Ground rules

- Every behaviour change comes with a test; coverage is gated in CI.
- The root module depends only on [goshard](https://github.com/darxnet/goshard);
  the generator and the SQLite tests live in their own modules so that their
  dependencies never reach users of the library.
- The router is allocation-free; keep it that way (the benchmarks show it).
