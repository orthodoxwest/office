ARG RUST_VERSION=1.94.1
FROM rust:${RUST_VERSION}-bookworm AS builder
WORKDIR /usr/src/app
COPY . .
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/usr/local/cargo/git \
    --mount=type=cache,target=/usr/src/app/target \
    cargo build --locked --release -p office-cli && \
    cp target/release/office /run-app

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /run-app /app/run-app
COPY --from=builder /usr/src/app/data /app/data
CMD ["/app/run-app", "serve"]
