# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "-s -w -X github.com/ryan/mkdone/internal/app.Version=${VERSION}" \
    -o /out/mkdone .

FROM build AS test
RUN go test ./...

FROM alpine:3.21 AS runtime
RUN apk add --no-cache ca-certificates ripgrep
COPY --from=build /out/mkdone /usr/local/bin/mkdone
WORKDIR /work
ENTRYPOINT ["mkdone"]
CMD ["--help"]
