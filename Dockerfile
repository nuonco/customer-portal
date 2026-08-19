ARG BUILD_IMAGE=buildimage
ARG RUNTIME_IMAGE=runtimeimage

# --- React client dependencies -------------------------------------------------
#
# Pulled from the ECR mirror rather than Docker Hub to avoid rate limits, matching
# services/wiki and nuon/services/dashboard-ui.
#
# package.json and bun.lock are copied on their own first so that the (slow)
# install layer is only invalidated when dependencies actually change, not on
# every source edit.

FROM 821160992543.dkr.ecr.us-west-2.amazonaws.com/docker.io/oven/bun:1.3-alpine AS client-deps

WORKDIR /src/ui

COPY ui/package.json ui/bun.lock ui/bunfig.toml ./

RUN --mount=type=cache,target=/root/.bun/install/cache,id=bun-customer-dashboard \
    bun install --frozen-lockfile

COPY ui/ ./

# --- React client lint / test targets ------------------------------------------

FROM client-deps AS client-lint

# Type checking lives here rather than in client-build so that a type error fails
# CI without also blocking an image build. This mirrors dashboard-ui, where the
# build stage runs only the bundler.
RUN bunx --bun oxlint . && bunx tsc --noEmit

FROM client-deps AS client-test

RUN bun test

# --- Build the React client ----------------------------------------------------

FROM client-deps AS client-build

# Deliberately `vite build` and not `bun run build`: the package script also runs
# `tsc --noEmit`, which is covered by the client-lint target above.
RUN --mount=type=cache,target=/root/.bun/install/cache,id=bun-customer-dashboard \
    bunx --bun vite build

# --- Go service ----------------------------------------------------------------

FROM ${BUILD_IMAGE} AS code

# Service specific arguments
ARG SERVICE=customer-dashboard
ARG PKG_ROOT=github.com/nuonco/mono/services/customer-dashboard

# Copy service-specific files
WORKDIR /src/services/${SERVICE}

COPY internal ./internal/
COPY pkg ./pkg/
COPY main.go ./
COPY static ./static/
COPY scripts ./scripts/
# Cache-busting hashes are derived from the on-disk CSS at request time
# (see internal/assets/assets.go); no hashed copies or manifest are generated.
RUN --mount=type=cache,target=/go/pkg/build-cache,id=gobuild-customer-dashboard \
    go generate ./...

FROM code AS build

RUN --mount=type=cache,target=/go/pkg/build-cache,id=gobuild-customer-dashboard \
    CGO_ENABLED=0 GOOS=linux go build -o /bin/service

FROM code AS test
RUN --mount=type=cache,target=/go/pkg/build-cache,id=gobuild-customer-dashboard \
    go test ./...

FROM code AS lint

COPY .golangci.yml .

RUN --mount=type=cache,target=/root/.cache/golangci-lint,id=golint-customer-dashboard \
    --mount=type=cache,target=/go/pkg/build-cache,id=gobuild-customer-dashboard \
    golangci-lint run -c .golangci.yml -v

# --- Final image ---------------------------------------------------------------

FROM ${RUNTIME_IMAGE} AS final

WORKDIR /app

ENV PATH=/go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/root/.local/bin

# expected as --build-args flags
ARG VERSION
ARG GIT_REF
ENV VERSION=$VERSION
ENV DD_VERSION=$VERSION
ENV GIT_REF=$GIT_REF

# Copy static assets (compiled CSS is hashed at request time by the server)
COPY --from=build /src/services/customer-dashboard/static /app/static

# React SPA bundle. DIST_DIR is read by internal/spa, which serves index.html as
# the fallback for client-side routes.
COPY --from=client-build /src/ui/dist /app/dist
ENV DIST_DIR=./dist

COPY --from=build /bin/service /bin/service

EXPOSE 8080

ENTRYPOINT ["/bin/service"]
