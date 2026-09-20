FROM golang:alpine AS builder

WORKDIR /app

# Download dependencies first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build all three binaries
RUN go build -o /bin/api ./cmd/api/main.go
RUN go build -o /bin/worker ./cmd/worker/main.go
RUN go build -o /bin/setup ./cmd/setup/main.go

# Use a minimal alpine image for the final stage
FROM alpine:latest
WORKDIR /app

# Copy binaries
COPY --from=builder /bin/api /bin/api
COPY --from=builder /bin/worker /bin/worker
COPY --from=builder /bin/setup /bin/setup

# Copy necessary static assets and configs
COPY configs/ ./configs/
COPY web/ ./web/
COPY .env.example ./.env

EXPOSE 8080
