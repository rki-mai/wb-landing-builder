# Build stage

FROM golang:1.25.10-alpine AS builder

RUN apk add --no-cache curl nodejs npm

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o wb-landing-builder .

# Pin CLI release explicitly; bump when landing-builder-cli interface changes.
# v2.0 = Go + Astro landing-builder-cli.
ARG LANDING_BUILDER_CLI_VERSION=v2.0
WORKDIR /opt/cli-install
RUN curl -fsSL "https://github.com/rki-mai/landing-builder-cli/releases/download/${LANDING_BUILDER_CLI_VERSION}/landing-builder-cli" \
    -o landing-builder-cli \
    && chmod +x landing-builder-cli \
    && ./landing-builder-cli install

# Runtime stage

FROM alpine:latest AS runtime

RUN apk add --no-cache nodejs npm curl

WORKDIR /app/
COPY --from=builder /app/wb-landing-builder .
COPY --from=builder /app/storage/schema.json storage/schema.json
COPY --from=builder /opt/cli-install /app/cli
CMD ["/app/wb-landing-builder"]
