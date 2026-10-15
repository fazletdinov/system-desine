# Для урока 1.16, не для продакшена: образ «как пишут в первый раз», чтобы было с чем
# сравнить. Контекст сборки — messenger/, .dockerignore тот же:
#   docker build -f ../course/modules/01-go-service/examples/dockerfiles/naive.Dockerfile .
# Что здесь не так: одна стадия (в образе остаются компилятор, исходники и кэш модулей),
# COPY . . до скачивания зависимостей (любая правка — модули качаются заново), нет cache
# mount, процесс работает от root, CGO включён по умолчанию.
FROM golang:1.26.9-trixie@sha256:f06b7122f888680048afb247c4e5f49c0069eb6c4c1ccae0f3f4834527a8d22c
WORKDIR /src
COPY . .
RUN go build -o /messenger ./cmd/messenger
CMD ["/messenger"]
