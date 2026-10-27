# syntax=docker/dockerfile:1
FROM golang:1.27-alpine AS build-go
WORKDIR /src
COPY core/go.mod core/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY core/ .
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/gitorgasm ./cmd/gitorgasm

FROM rust:1.97-bookworm AS build-score
WORKDIR /src
COPY scoring/Cargo.toml scoring/Cargo.lock ./
COPY scoring/src ./src
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    cargo build --release --locked --bin score && cp target/release/score /out-score

FROM gcr.io/distroless/cc-debian12:nonroot
COPY --from=build-go /out/gitorgasm /gitorgasm
COPY --from=build-score /out-score /gitorgasm-score
USER 65532:65532
EXPOSE 8080 9090
ENTRYPOINT ["/gitorgasm"]
CMD ["api"]
