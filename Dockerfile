# Keep this tag and its digest in step with go.mod's toolchain line, which is the one statement of the Go version.
FROM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/frappe-mcp-server .

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=build /out/frappe-mcp-server ./
COPY LICENSE THIRD_PARTY_NOTICES ./
USER 1001
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/health >/dev/null || exit 1
CMD ["./frappe-mcp-server"]
