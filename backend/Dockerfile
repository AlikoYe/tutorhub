# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate
# The uploads volume inherits this directory's owner (distroless nonroot = 65532) on first mount.
RUN mkdir -p /out/data/uploads

# distroless/static ships CA certificates (needed for api.telegram.org) and
# tzdata, runs as non-root, and has no shell.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/api /out/migrate /app/
COPY --from=build --chown=65532:65532 /out/data /data
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/api"]
