# Build stage

FROM golang:1.25.10-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o wb-landing-builder .

# Pin CLI commit explicitly; bump when landing-builder-cli interface changes.
ARG LANDING_BUILDER_CLI_COMMIT=e05e2b1
RUN git clone https://github.com/rki-mai/landing-builder-cli.git /opt/cli \
    && git -C /opt/cli checkout "${LANDING_BUILDER_CLI_COMMIT}"

# Runtime stage

FROM alpine:latest AS runtime

RUN apk add --no-cache python3 curl

WORKDIR /app/
COPY --from=builder /app/wb-landing-builder .
COPY --from=builder /app/storage/schema.json storage/schema.json
COPY --from=builder /opt/cli /app/cli
CMD ["/app/wb-landing-builder"]
