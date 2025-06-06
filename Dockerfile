FROM --platform=$BUILDPLATFORM tonistiigi/xx:1.6.1 AS xx


# Frontend build
FROM --platform=$BUILDPLATFORM node:24-alpine AS ui-builder
WORKDIR /app/ui

COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ .
RUN npm run build


FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS go-base
WORKDIR /app
COPY --from=xx / /
RUN apk add --no-cache git


# Rmapi build
FROM --platform=$BUILDPLATFORM go-base AS rmapi-source
RUN git clone --branch v0.0.30 https://github.com/ddvk/rmapi .

FROM --platform=$BUILDPLATFORM go-base AS rmapi-builder

COPY --from=rmapi-source /app/go.mod /app/go.sum ./
RUN go mod download

COPY --from=rmapi-source /app .
ARG TARGETPLATFORM
RUN --mount=type=cache,target=/root/.cache \
    CGO_ENABLED=0 xx-go build -ldflags='-w -s' -trimpath


# Aviary build
FROM --platform=$BUILDPLATFORM go-base AS aviary-builder

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=ui-builder /app/ui/out ./ui/out

ARG TARGETPLATFORM
RUN --mount=type=cache,target=/root/.cache \
    CGO_ENABLED=0 xx-go build -ldflags='-w -s' -trimpath


# Final image
FROM alpine:3.22

# Install runtime dependencies
RUN apk add --no-cache \
      ca-certificates \
      ghostscript \
      imagemagick \
    && update-ca-certificates

WORKDIR /app

COPY --from=rmapi-builder /app/rmapi /usr/local/bin/
COPY --from=aviary-builder /app/aviary /usr/local/bin/

ENV PORT=8000 \
    PDF_DIR=/app/pdfs \
    RM_TARGET_DIR=/ \
    GS_COMPAT=1.4 \
    GS_SETTINGS=/ebook

ENTRYPOINT ["/usr/local/bin/aviary"]
