# syntax=docker/dockerfile:1
FROM golang:1.25-bookworm AS builder
ARG VERSION=docker
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o /ctx ./cmd/ctx

FROM gcr.io/distroless/static-debian12
COPY --from=builder /ctx /usr/local/bin/ctx
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/ctx"]
