# Build stage
FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o guardian cmd/guardian/main.go

# Final stage
FROM alpine:3.21
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/guardian .
ENTRYPOINT ["/app/guardian"]
