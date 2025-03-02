# Build stage (Debian-based Go image)
FROM golang:1.24-bullseye AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN GOOS=linux GOARCH=amd64 go build -o myapp

FROM debian:bullseye-slim
WORKDIR /app

# Install certificates if needed
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/myapp /app/myapp
COPY templates/ /app/templates/

EXPOSE 8080
CMD ["/app/myapp"]
