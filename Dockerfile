FROM --platform=${BUILDPLATFORM} golang:1.16-alpine AS base

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
ENV CGO_ENABLED=0
COPY go.* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

FROM base AS build

ARG TARGETOS
ARG TARGETARCH
RUN --mount=target=. \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o /out/alertdog .

FROM scratch AS bin
COPY --from=build /out/alertdog /
ENTRYPOINT ["/alertdog"]
