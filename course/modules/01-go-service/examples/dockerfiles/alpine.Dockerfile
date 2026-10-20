# syntax=docker/dockerfile:1.19@sha256:b6afd42430b15f2d2a4c5a02b919e98a525b785b1aaff16747d2f623364e39b6

# Для урока 1.16, не для продакшена: тот же бинарь на alpine — для сравнения размера,
# числа пакетов и CVE с distroless. Контекст сборки — messenger/, .dockerignore тот же.
# Стадия сборки — как в messenger/Dockerfile.
FROM --platform=$BUILDPLATFORM golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
ARG TARGETOS TARGETARCH
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/messenger ./cmd/messenger

# alpine: musl libc, busybox (sh и сотни утилит), пакетный менеджер apk, CA-сертификаты.
# Пользователя без прав нет — его приходится заводить самим.
FROM alpine:3.23@sha256:85fe1e81d6758c208f3e1eed4338a1997e19d4be002d4dd32d3100c9a8c010a0
RUN adduser -D -u 65532 app
COPY --from=build /out/messenger /messenger
ENV MESSENGER_ADDR=:8080
EXPOSE 8080
USER 65532
ENTRYPOINT ["/messenger"]
