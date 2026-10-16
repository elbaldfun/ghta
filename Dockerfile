# ---- build ----
# Must match (or exceed) the go directive in go.mod, otherwise the build fails
# or silently pulls a toolchain at build time.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# One image, two entrypoints: the api and the worker (docs/REBUILD.md) always
# ship from the same commit.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/api ./cmd/worker ./cmd/import

# ---- run ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
# taxonomy.Load reads "taxonomy/taxonomy.yaml" relative to the working dir at
# startup, so the YAML has to ship alongside the binary.
COPY --from=build /src/taxonomy /app/taxonomy
COPY --from=build /out/api /app/api
COPY --from=build /out/worker /app/worker
COPY --from=build /out/import /app/import
USER nonroot:nonroot
EXPOSE 3000
ENTRYPOINT ["/app/api"]
