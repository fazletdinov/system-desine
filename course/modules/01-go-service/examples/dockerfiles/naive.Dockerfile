# Для урока 1.16, не для продакшена: образ «как пишут в первый раз», чтобы было с чем
# сравнить. Контекст сборки — messenger/, .dockerignore тот же:
#   docker build -f ../course/modules/01-go-service/examples/dockerfiles/naive.Dockerfile .
# Что здесь не так: одна стадия (в образе остаются компилятор, исходники и кэш модулей),
# COPY . . до скачивания зависимостей (любая правка — модули качаются заново), нет cache
# mount, процесс работает от root, CGO включён по умолчанию.
FROM golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e
WORKDIR /src
COPY . .
RUN go build -o /messenger ./cmd/messenger
CMD ["/messenger"]
