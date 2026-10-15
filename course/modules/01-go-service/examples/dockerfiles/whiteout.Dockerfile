# Для урока 1.16, не для продакшена: два антипримера в одном образе. Контекст не нужен:
#   docker build -t whiteout-demo --build-arg API_TOKEN=s3cr3t-token -f whiteout.Dockerfile .
# 1) rm в отдельной инструкции не уменьшает образ: 50 МБ остаются в нижнем слое,
#    сверху ложится только whiteout-метка;
# 2) значение ARG записывается в историю образа открытым текстом (docker history).
FROM alpine:3.23@sha256:85fe1e81d6758c208f3e1eed4338a1997e19d4be002d4dd32d3100c9a8c010a0
RUN head -c 50000000 /dev/urandom > /big.bin
RUN rm /big.bin
ARG API_TOKEN
RUN echo "токен длиной ${#API_TOKEN}" > /dev/null
