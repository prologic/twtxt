# Build
FROM golang:alpine AS build

RUN apk add --no-cache -U build-base git make

RUN mkdir /src

WORKDIR /src

# Copy Makefile
COPY Makefile ./

# Copy go.mod and go.sum and install and cache dependencies
COPY go.mod .
COPY go.sum .

# Install deps
RUN go get github.com/GeertJohan/go.rice/rice
RUN go mod download

# Copy static assets
COPY ./static/css/* ./static/css/
COPY ./static/img/* ./static/img/
COPY ./static/js/* ./static/js/

# Copy templates
COPY ./templates/* ./templates/

# Copy sources
COPY *.go ./
COPY ./auth/*.go ./auth/
COPY ./session/*.go ./session/
COPY ./password/*.go ./password/
COPY ./cmd/twtd/*.go ./cmd/twtd/

# Version/Commit (there there is no .git in Docker build context)
# NOTE: This is fairly low down in the Dockerfile instructions so
#       we don't break the Docker build cache just be changing
#       unrelated files that actually haven't changed but caused the
#       COMMIT value to change.
ARG VERSION="0.0.0"
ARG COMMIT="HEAD"

# Build binary
RUN make build VERSION=$VERSION COMMIT=$COMMIT

# Runtime
FROM alpine:latest

RUN apk --no-cache -U add ca-certificates

WORKDIR /
VOLUME /data

COPY --from=build /src/twtd /twtd

ENTRYPOINT ["/twtd"]
