# ---- build ----
# Must match (or exceed) the go directive in go.mod, otherwise the build fails
# or silently pulls a toolchain at build time.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---- run ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
# taxonomy.Load reads "taxonomy/taxonomy.yaml" relative to the working dir at
# startup, so the YAML has to ship alongside the binary.
COPY --from=build /src/taxonomy /app/taxonomy
COPY --from=build /out/api /app/api
USER nonroot:nonroot
EXPOSE 3000
ENTRYPOINT ["/app/api"]
