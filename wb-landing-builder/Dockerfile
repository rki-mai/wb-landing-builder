# Build stage

FROM golang:1.25.10-alpine AS builder

RUN apk add --no-cache git nodejs npm

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o wb-landing-builder .

# Pin CLI commit explicitly; bump when landing-builder-cli interface changes.
# v2.0 = merge of PR #4 (Go + Astro landing-builder-cli).
ARG LANDING_BUILDER_CLI_COMMIT=1a7cf2d
RUN git clone https://github.com/rki-mai/landing-builder-cli.git /opt/cli-src \
    && git -C /opt/cli-src checkout "${LANDING_BUILDER_CLI_COMMIT}"

WORKDIR /opt/cli-src/landing-builder-cli
ENV GOTOOLCHAIN=auto
RUN CGO_ENABLED=0 GOOS=linux go build -o /opt/landing-builder-cli .

WORKDIR /opt/cli-install
RUN cp /opt/landing-builder-cli . \
    && ./landing-builder-cli install

# Runtime stage

FROM alpine:latest AS runtime

RUN apk add --no-cache nodejs npm curl

WORKDIR /app/
COPY --from=builder /app/wb-landing-builder .
COPY --from=builder /app/storage/schema.json storage/schema.json
COPY --from=builder /opt/cli-install/landing-builder-cli /app/cli/landing-builder-cli
COPY --from=builder /opt/cli-install/.template-project /app/cli/.template-project
CMD ["/app/wb-landing-builder"]
