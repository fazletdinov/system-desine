# syntax=docker/dockerfile:1.19@sha256:b6afd42430b15f2d2a4c5a02b919e98a525b785b1aaff16747d2f623364e39b6

# Для урока 1.16, не для продакшена: тот же бинарь на пустом образе — для сравнения
# с distroless. Контекст сборки — messenger/, .dockerignore тот же.
# Стадия сборки — как в messenger/Dockerfile.
FROM --platform=$BUILDPLATFORM golang:1.26.9-trixie@sha256:f06b7122f888680048afb247c4e5f49c0069eb6c4c1ccae0f3f4834527a8d22c AS build
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

# scratch — пустой образ: всё, что нужно процессу, кладём сами. CA-сертификаты (для
# исходящего HTTPS) и tzdata (для time.LoadLocation) берём из стадии сборки.
# Чего не хватает по сравнению с distroless static:
# - /etc/passwd и /etc/group: UID 65532 есть только числом, у него нет имени и домашнего
#   каталога, и os/user.Current() без $USER в окружении вернёт ошибку;
# - /tmp: os.TempDir() вернёт /tmp, которого нет, и os.CreateTemp упадёт;
# - /home/nonroot, /etc/nsswitch.conf, /etc/os-release и данные о пакетах для сканера.
# Для Go-сервиса это обычно терпимо: статическому бинарю не нужны ни libc, ни shell,
# а имя пользователя и временные файлы мессенджеру не нужны. Но каждую такую мелочь
# приходится помнить самим — distroless собирает их за нас.
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /out/messenger /messenger
ENV MESSENGER_ADDR=:8080
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/messenger"]
