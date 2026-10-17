FROM golang:1.25-alpine AS builder

ARG VERSION=dev
ARG BRANCH=unknown
ARG REVISION=unknown
ARG BUILD_TIME=unknown

ENV CGO_ENABLED=0 \
    GOWORK=off \
    GO111MODULE=on

WORKDIR /build

COPY cs-im-server/go.mod cs-im-server/go.sum ./cs-im-server/
COPY cs-im-common/go.mod cs-im-common/go.sum ./cs-im-common/
COPY cs-im-common/ ./cs-im-common/
COPY cs-im-server/ ./cs-im-server/

RUN --mount=type=cache,target=/go/pkg/mod \
    cd cs-im-server && go mod download

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd cs-im-server && go build -trimpath \
    -ldflags="-w -s \
        -X main.Version=${VERSION} \
        -X main.Branch=${BRANCH} \
        -X main.Revision=${REVISION} \
        -X main.BuildTime=${BUILD_TIME}" \
    -o /out/cs-im-server ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata busybox-extras
WORKDIR /app
COPY --from=builder /out/cs-im-server /usr/local/bin/cs-im-server
COPY cs-im-server/config/config.yaml /app/config.yaml
ENV TZ=Asia/Shanghai
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD nc -z 127.0.0.1 8080 || exit 1
ENTRYPOINT ["/usr/local/bin/cs-im-server"]
CMD ["start", "-c", "/app/config.yaml"]
