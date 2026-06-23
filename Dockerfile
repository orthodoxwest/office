ARG GO_VERSION=1.26.3
FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /usr/src/app
COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o /run-app ./cmd/server


FROM scratch

COPY --from=builder /run-app /app/run-app
COPY --from=builder /usr/src/app/data /app/data

CMD ["/app/run-app", "serve"]
