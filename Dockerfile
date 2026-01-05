#
# Build stage
#

FROM debian:bookworm-slim AS builder

# --- Install a clean Go toolchain into /opt/go ---
ARG GO_VERSION=1.25.5
ARG GO_TARBALL=go${GO_VERSION}.linux-amd64.tar.gz
ARG GO_SHA256=9e9b755d63b36acf30c12a9a3fc379243714c1c6d3dd72861da637f336ebb35b

RUN apt-get update && \
    rm -f /etc/ssl/openssl.cnf || true && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      ca-certificates curl git xz-utils && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app
RUN curl -fsSLO https://go.dev/dl/${GO_TARBALL} \
 && echo "${GO_SHA256}  ${GO_TARBALL}" | sha256sum -c -

RUN mkdir -p /opt && tar -C /opt -xzf ${GO_TARBALL} && mv /opt/go /opt/go${GO_VERSION}

# Go env (avoid any runner overlays)
ENV GOROOT=/opt/go${GO_VERSION}
ENV GOPATH=/go
ENV PATH=$GOROOT/bin:$GOPATH/bin:$PATH
ENV GOTOOLCHAIN=local
ENV GOEXPERIMENT=
ENV GOFLAGS=
ENV GOPROXY=https://proxy.golang.org,direct
ENV GOMODCACHE=/go/pkg/mod

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o installer-app .

#
# Runtime stage
#

FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS calls to Nuon API
RUN apk add --no-cache ca-certificates tzdata

# Copy binary and assets
COPY --from=builder /app/installer-app .
COPY --from=builder /app/internal/templates ./internal/templates
COPY --from=builder /app/static ./static

# Expose ports for vendor (8080) and customer (8081) portals
EXPOSE 8080 8081

# Run the application
CMD ["./installer-app"]
