# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /build

# Copy SDK
COPY ../.. /build/sdk
WORKDIR /build/sdk/examples/echo

# Download dependencies
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o echo-app .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /build/sdk/examples/echo/echo-app .

CMD ["./echo-app"]
