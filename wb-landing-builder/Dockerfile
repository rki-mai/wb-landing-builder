# Build stage

FROM golang:1.25.10-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o wb-landing-builder .

# Runtime stage

FROM alpine:latest AS runtime

WORKDIR /app/
COPY --from=builder /app/wb-landing-builder .
CMD ["/app/wb-landing-builder"]
