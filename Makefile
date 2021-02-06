-include environ.inc
.PHONY: deps dev build install image release test clean

CGO_ENABLED=0
VERSION=$(shell git describe --abbrev=0 --tags 2>/dev/null || echo "$VERSION")
COMMIT=$(shell git rev-parse --short HEAD || echo "$COMMIT")

all: build

deps:
	@go get -u github.com/GeertJohan/go.rice/rice
	@go get -u github.com/tdewolff/minify/v2/cmd/...

dev : DEBUG=1
dev : build
	@./twt -v
	./twtd -D -O -R $(FLAGS)

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
	@if [ x"$(DEBUG)" = x"1"  ]; then		\
	  echo 'Running in debug mode...';	\
	  rm -f -v ./internal/rice-box.go;	\
	else								\
	  minify -b -o ./internal/static/css/twtxt.min.css ./internal/static/css/[0-9]*-*.css;	\
	  minify -b -o ./internal/static/js/twtxt.min.js ./internal/static/js/[0-9]*-*.js;		\
	  rm -f ./internal/rice-box.go;	\
	  rice -i ./internal embed-go;	\
	fi

install: build
	@go install ./cmd/twt/...
	@go install ./cmd/twtd/...

ifeq ($(PUBLISH), 1)
image:
	@docker build --build-arg VERSION="$(VERSION)" --build-arg COMMIT="$(COMMIT)" -t jointwt/twtxt .
	@docker push jointwt/twtxt
	# TODO: Remove at some point
	@docker tag jointwt/twtxt prologic/twtxt
	@docker push prologic/twtxt
else
image:
	@docker build --build-arg VERSION="$(VERSION)" --build-arg COMMIT="$(COMMIT)" -t jointwt/twtxt .
endif

release:
	@./tools/release.sh

test:
	@go test -v -cover -race ./...

bench: bench-twtxt.txt
	go test -race -benchtime=1x -cpu 16 -benchmem -bench "^(Benchmark)" github.com/jointwt/twtxt/types

bench-twtxt.txt:
	curl -s https://twtxt.net/user/prologic/twtxt.txt > $@

clean:
	@git clean -f -d -X
