# Build stage (Debian-based Go image)
FROM golang:1.27.0-trixie AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /out/rsvp ./cmd/web \
    && go build -o /out/llm-proxy-fixture ./internal/llmproxyfixture

FROM debian:trixie-slim AS runtime-base
WORKDIR /app

# Install certificates if needed
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

FROM runtime-base AS llm-proxy-fixture
COPY --from=builder /out/llm-proxy-fixture /app/llm-proxy-fixture
CMD ["/app/llm-proxy-fixture"]

FROM debian:trixie-slim AS website-builder
WORKDIR /site
COPY static/index.html /site/index.html
COPY static/ /site/static/
COPY configs/ui-production.yaml /site/config-ui.yaml
RUN sed -i 's|data-workspace-origin="/"|data-workspace-origin="https://rsvp-api.mprlab.com/"|' /site/index.html \
    && rm /site/static/index.html

FROM scratch AS website
COPY --from=website-builder /site/ /

FROM runtime-base AS runtime
COPY --from=builder /out/rsvp /app/rsvp
COPY templates/ /app/templates/
COPY config.yml /app/config.yml

EXPOSE 8080
CMD ["/app/rsvp"]
