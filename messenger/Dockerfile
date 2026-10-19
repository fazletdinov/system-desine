# syntax=docker/dockerfile:1.19@sha256:b6afd42430b15f2d2a4c5a02b919e98a525b785b1aaff16747d2f623364e39b6

# region build
# Стадия сборки идёт на платформе сборщика ($BUILDPLATFORM): Go сам кросс-компилирует
# под TARGETOS/TARGETARCH, и для arm64-образа не нужна эмуляция QEMU (урок 1.19).
FROM --platform=$BUILDPLATFORM golang:1.27.0-trixie@sha256:df98008ecd2b0ecc9f0a94d1b07e3564a9c92b555369b33d9b5f60d0765b2db7 AS build
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
FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
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
