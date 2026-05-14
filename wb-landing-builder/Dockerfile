# Build stage

FROM golang:1.25.10-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o wb-landing-builder .

# Runtime stage

FROM alpine:latest AS runtime

WORKDIR /app/
COPY --from=builder /app/wb-landing-builder .
COPY --from=builder /app/storage/schema.json storage/schema.json
CMD ["/app/wb-landing-builder"]
