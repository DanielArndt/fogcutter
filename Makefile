# SPDX-FileCopyrightText: 2026 Daniel Arndt <dan@arndt.ca>
# SPDX-License-Identifier: Apache-2.0

BIN = fogcutter
PKG = ./cmd/fogcutter

.PHONY: all build test lint fmt clean

all: lint test build

# Builds the CLI into ./fogcutter
build:
	go build -o $(BIN) $(PKG)
	go build ./...

# Vets and runs the unit tests with the race detector
test:
	go vet ./...
	go test -race ./...

# Fails on unformatted files, then runs golangci-lint (requires golangci-lint v2)
lint:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "These files need gofmt (run 'make fmt'):"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	golangci-lint run ./...

# Formats all Go files in place
fmt:
	gofmt -w .

clean:
	rm -f $(BIN)
