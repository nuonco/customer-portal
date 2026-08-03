ARG BUILD_IMAGE=buildimage
ARG RUNTIME_IMAGE=runtimeimage

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

# Use the provided runtime image
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
COPY --from=build /bin/service /bin/service

EXPOSE 8080

ENTRYPOINT ["/bin/service"]
