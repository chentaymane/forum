# ─── Build stage ──────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o forum .

# ─── Runtime stage ────────────────────────────────────────────────────────
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/forum .
COPY --from=builder /app/web ./web
COPY --from=builder /app/schema.sql .

EXPOSE 8080
CMD ["./forum"]
