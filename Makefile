.PHONY: deps dev build install image release test clean

CGO_ENABLED=0
VERSION=$(shell git describe --abbrev=0 --tags 2>/dev/null || echo "$VERSION")
COMMIT=$(shell git rev-parse --short HEAD || echo "$COMMIT")

all: dev

deps:
	@go get -u github.com/GeertJohan/go.rice/rice
	@go get -u github.com/tdewolff/minify/cmd/minify

dev: build
	@./twt -v
	@./twtd -D -O -R

cli:
	@go build -tags "netgo static_build" -installsuffix netgo \
		-ldflags "-w \
		-X $(shell go list).Version=$(VERSION) \
		-X $(shell go list).Commit=$(COMMIT)" \
		./cmd/twt/...

server: generate
	@go build -tags "netgo static_build" -installsuffix netgo \
		-ldflags "-w \
		-X $(shell go list).Version=$(VERSION) \
		-X $(shell go list).Commit=$(COMMIT)" \
		./cmd/twtd/...

build: cli server

generate:
	@rice -i ./internal embed-go
	@minify -o ./internal/static/css/twtxt.min.css ./internal/static/css/[0-9]*-*.css
	@minify -o ./internal/static/js/twtxt.min.js ./internal/static/js/[0-9]*-*.js

install: build
	@go install ./cmd/twt/...
	@go install ./cmd/twtd/...

image:
	@docker build --build-arg VERSION="$(VERSION)" --build-arg COMMIT="$(COMMIT)" -t prologic/twtxt .
	@docker push prologic/twtxt

release:
	@./tools/release.sh

test:
	@go test -v -cover -race ./...

clean:
	@git clean -f -d -X
