# Для урока 1.16, не для продакшена: образ «как пишут в первый раз», чтобы было с чем
# сравнить. Контекст сборки — messenger/, .dockerignore тот же:
#   docker build -f ../course/modules/01-go-service/examples/dockerfiles/naive.Dockerfile .
# Что здесь не так: одна стадия (в образе остаются компилятор, исходники и кэш модулей),
# COPY . . до скачивания зависимостей (любая правка — модули качаются заново), нет cache
# mount, процесс работает от root, CGO включён по умолчанию.
FROM golang:1.27.3-trixie@sha256:fbb7ae40b62ff8afe31662df570d39b4f5052ac32b90dae8e60d7657a20a74f4
WORKDIR /src
COPY . .
RUN go build -o /messenger ./cmd/messenger
CMD ["/messenger"]
