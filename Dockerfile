FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY . .
RUN go mod download && go build -o forum .

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/forum .
COPY --from=builder /app/web ./web
COPY --from=builder /app/schema.sql .

EXPOSE 8080
CMD ["./forum"]