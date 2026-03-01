# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev git

WORKDIR /app

# Copy all source files
COPY . .

# Resolve dependencies
RUN go mod download && go mod tidy

# Build the application
RUN go build -o forum .

# Run stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /root/

# Create database directory
RUN mkdir -p /root/database

# Copy the binary from the builder stage
COPY --from=builder /app/forum .
# Copy static and templates directories
COPY --from=builder /app/static ./static
COPY --from=builder /app/templates ./templates

# Expose port 8080
EXPOSE 8080

# Command to run the executable
CMD ["./forum"]
