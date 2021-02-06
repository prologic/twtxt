# Build
FROM golang:alpine AS build

RUN apk add --no-cache -U build-base git make ffmpeg-dev

RUN mkdir -p /src

WORKDIR /src

# Copy Makefile
COPY Makefile ./

# Copy go.mod and go.sum and install and cache dependencies
COPY go.mod .
COPY go.sum .

# Install deps
RUN make deps
RUN go mod download

# Copy static assets
COPY ./internal/static/css/* ./internal/static/css/
COPY ./internal/static/img/* ./internal/static/img/
COPY ./internal/static/js/* ./internal/static/js/

# Copy pages
COPY ./internal/pages/* ./internal/pages/

# Copy templates
COPY ./internal/templates/* ./internal/templates/

# Copy sources
COPY *.go ./
COPY ./internal/*.go ./internal/
COPY ./internal/auth/*.go ./internal/auth/
COPY ./internal/session/*.go ./internal/session/
COPY ./internal/passwords/*.go ./internal/passwords/
COPY ./internal/webmention/*.go ./internal/webmention/
COPY ./types/*.go ./types/
COPY ./types/retwt/*.go ./types/retwt/
COPY ./types/lextwt/*.go ./types/lextwt/
COPY ./cmd/twtd/*.go ./cmd/twtd/

# Version/Commit (there there is no .git in Docker build context)
# NOTE: This is fairly low down in the Dockerfile instructions so
#       we don't break the Docker build cache just be changing
#       unrelated files that actually haven't changed but caused the
#       COMMIT value to change.
ARG VERSION="0.0.0"
ARG COMMIT="HEAD"

# Build server binary
RUN make server VERSION=$VERSION COMMIT=$COMMIT

# Runtime
FROM alpine:latest

RUN apk --no-cache -U add ca-certificates tzdata ffmpeg

WORKDIR /
VOLUME /data

# force cgo resolver
ENV GODEBUG=netdns=cgo

COPY --from=build /src/twtd /twtd

ENTRYPOINT ["/twtd"]
CMD [""]
