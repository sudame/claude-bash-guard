BIN := bin/bash-guard

.PHONY: build clean test

build:
	@mkdir -p bin
	go build -trimpath -ldflags="-s -w" -o $(BIN) .

test:
	go test ./...

clean:
	rm -rf bin
