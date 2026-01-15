# Stage 1: Build the Go binary
FROM golang:alpine AS builder

# Install SSL certs (Required for calling Square/Lob APIs)
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Cache dependencies first (makes builds faster)
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the binary
# We explicitly disable CGO for a pure static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

# Stage 2: Create the final lightweight image
FROM alpine:latest

WORKDIR /root/

# Copy the SSL certs from the builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /app/server .

# Copy the "web" folder (HTML/CSS) because your code reads it at runtime
COPY --from=builder /app/web ./web

# Copy the binary
COPY --from=builder /app/server .

# Copy the "web" folder
COPY --from=builder /app/web ./web

# [FIX] Copy the favicon
COPY --from=builder /app/favicon.ico .

# Expose the port
EXPOSE 8080

# Run it
CMD ["./server"]