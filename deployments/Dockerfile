# ============================================================
# Multi-stage build
# ============================================================
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git for go get (if needed)
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Generate Swagger docs
RUN go install github.com/swaggo/swag/cmd/swag@latest && \
    swag init -g cmd/server/main.go -o api/docs

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/population-service ./cmd/server

# ============================================================
# Runtime stage
# ============================================================
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Ho_Chi_Minh

WORKDIR /app

COPY --from=builder /bin/population-service .
COPY --from=builder /app/configs/.env.example ./configs/.env.example
COPY --from=builder /app/scripts ./scripts

EXPOSE 8080

CMD ["./population-service"]
