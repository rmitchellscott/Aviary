FROM --platform=$BUILDPLATFORM tonistiigi/xx:1.9.0 AS xx


# Frontend build
FROM --platform=$BUILDPLATFORM node:24-alpine AS ui-builder
WORKDIR /app

COPY ui/package.json ui/package-lock.json ui/
RUN cd ui && npm ci
COPY locales/ locales/
COPY ui/ ui/
RUN cd ui && npm run build


FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS go-base
WORKDIR /app
COPY --from=xx / /
RUN apk add --no-cache git

# Rmapi stage
FROM ghcr.io/ddvk/rmapi:v0.0.32 AS rmapi-binary

# Aviary build
FROM --platform=$BUILDPLATFORM go-base AS aviary-builder

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=ui-builder /app/ui/dist ./ui/dist

# Build args for version injection
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown
ARG TARGETPLATFORM

RUN --mount=type=cache,target=/root/.cache \
    CGO_ENABLED=0 xx-go build \
    -ldflags="-w -s \
        -X github.com/rmitchellscott/aviary/internal/version.Version=${VERSION} \
        -X github.com/rmitchellscott/aviary/internal/version.GitCommit=${GIT_COMMIT} \
        -X github.com/rmitchellscott/aviary/internal/version.BuildDate=${BUILD_DATE}" \
    -trimpath


# Final image
FROM alpine:3.22

# Install runtime dependencies
RUN apk add --no-cache \
      ca-certificates \
      ghostscript \
      imagemagick \
      postgresql-client \
      mupdf-tools \
    && update-ca-certificates

WORKDIR /app

COPY --from=rmapi-binary /usr/local/bin/rmapi /usr/local/bin/
COPY --from=aviary-builder /app/aviary /usr/local/bin/
COPY --from=aviary-builder /app/locales ./locales

ENV PORT=8000 \
    PDF_DIR=/app/pdfs \
    RM_TARGET_DIR=/ \
    GS_COMPAT=1.7 \
    GS_SETTINGS=/ebook

ENTRYPOINT ["/usr/local/bin/aviary"]
