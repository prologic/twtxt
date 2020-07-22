# Build
FROM golang:alpine AS build

RUN apk add --no-cache -U build-base git make

RUN mkdir /src

WORKDIR /src

COPY ./src/Makefile .
RUN make deps

COPY ./src .
RUN make build

# Runtime
FROM alpine:latest

RUN apk --no-cache -U add ca-certificates

WORKDIR /
VOLUME /data

COPY --from=build /src/twtd /twtd

ENTRYPOINT ["/twtd"]
