# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Instalar certificados e dependências de build
RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/api ./cmd/api

# Final stage
FROM alpine:3.20

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/bin/api /app/api

EXPOSE 8080

ENTRYPOINT ["/app/api"]
