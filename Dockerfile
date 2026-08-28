# Build stage (Debian-based Go image)
FROM golang:1.27.0-trixie AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /out/rsvp ./cmd/web \
    && go build -o /out/natural-language-fixture ./internal/naturallanguagefixture

FROM debian:trixie-slim AS runtime-base
WORKDIR /app

# Install certificates if needed
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

FROM runtime-base AS natural-language-parser
COPY --from=builder /out/natural-language-fixture /app/natural-language-fixture
CMD ["/app/natural-language-fixture"]

FROM runtime-base AS runtime
COPY --from=builder /out/rsvp /app/rsvp
COPY templates/ /app/templates/

EXPOSE 8080
CMD ["/app/rsvp"]
