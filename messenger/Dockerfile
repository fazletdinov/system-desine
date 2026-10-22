# syntax=docker/dockerfile:1.19@sha256:b6afd42430b15f2d2a4c5a02b919e98a525b785b1aaff16747d2f623364e39b6

# region build
# Стадия сборки идёт на платформе сборщика ($BUILDPLATFORM): Go сам кросс-компилирует
# под TARGETOS/TARGETARCH, и для arm64-образа не нужна эмуляция QEMU (урок 1.19).
FROM --platform=$BUILDPLATFORM golang:1.27.3-trixie@sha256:fbb7ae40b62ff8afe31662df570d39b4f5052ac32b90dae8e60d7657a20a74f4 AS build
WORKDIR /src
# Сначала только манифесты модулей: слой зависимостей пересобирается, когда меняются
# они, а не при каждой правке кода.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
ARG TARGETOS TARGETARCH
ARG VERSION=dev
# CGO_ENABLED=0 — статический бинарь без libc: он запустится в образе, где libc нет.
# -trimpath убирает пути машины сборщика (воспроизводимость), -s -w — таблицы отладки.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/messenger ./cmd/messenger
# endregion build

# region runtime
# distroless static: CA-сертификаты, tzdata, /etc/passwd с пользователем nonroot (65532)
# — и больше ничего: ни shell, ни пакетного менеджера (ADR-0007).
FROM gcr.io/distroless/static-debian13:nonroot@sha256:d62067879de890a1def9da0e7766b7d20810a5e5555cef8506b752437d7a3e88
LABEL org.opencontainers.image.title="messenger" \
      org.opencontainers.image.description="Мессенджер курса «System Design на практике»: монолит v0"
COPY --from=build /out/messenger /messenger
# Внутри контейнера loopback недоступен снаружи: слушаем все интерфейсы. Наружу порт
# откроет только явный -p / ports: (урок 1.17).
ENV MESSENGER_ADDR=:8080
EXPOSE 8080
USER 65532:65532
# region healthcheck
# В distroless нет curl и shell — проверку делает сам бинарь. start-interval (Docker 25+)
# проверяет чаще, пока контейнер стартует: healthy через секунду, а не через interval.
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --start-interval=1s --retries=3 \
    CMD ["/messenger", "healthcheck"]
# endregion healthcheck
# Exec-форма: messenger — PID 1 и сам получает SIGTERM (урок 1.17).
ENTRYPOINT ["/messenger"]
# endregion runtime
