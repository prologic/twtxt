.PHONY: deps dev build install image release test clean

CGO_ENABLED=0
VERSION=$(shell git describe --abbrev=0 --tags 2>/dev/null || echo "$VERSION")
COMMIT=$(shell git rev-parse --short HEAD || echo "$COMMIT")

all: dev

deps:
	@go get github.com/GeertJohan/go.rice/rice

dev: build
	@./twtd -D -r

build: generate
	@go build -tags "netgo static_build" -installsuffix netgo \
		-ldflags "-w -X $(shell go list).Version=$(VERSION) -X $(shell go list).Commit=$(COMMIT)" \
		./cmd/twtd/...

generate:
	@rice -i ./internal embed-go

install: build
	@go install

image:
	@docker build -t prologic/twtxt .

release:
	@./tools/release.sh

test:
	@go test -v -cover -race .

clean:
	@git clean -f -d -X
