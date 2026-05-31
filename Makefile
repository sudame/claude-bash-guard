BIN_GUARD := bin/bash-guard
BIN_BOTPR := bin/botpr
BIN_GHBOT := bin/ghbot
GOFLAGS := -trimpath -ldflags="-s -w"

.PHONY: build clean test

build:
	@mkdir -p bin
	go build $(GOFLAGS) -o $(BIN_GUARD) ./cmd/bash-guard
	go build $(GOFLAGS) -o $(BIN_BOTPR) ./cmd/botpr
	go build $(GOFLAGS) -o $(BIN_GHBOT) ./cmd/ghbot

test:
	go test ./...

clean:
	rm -rf bin
