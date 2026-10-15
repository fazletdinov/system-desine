# syntax=docker/dockerfile:1.19@sha256:b6afd42430b15f2d2a4c5a02b919e98a525b785b1aaff16747d2f623364e39b6

# Для урока 1.17, не для продакшена: кто становится PID 1 и получает SIGTERM.
# Бинарь берём из уже собранного образа мессенджера (docker compose build в messenger/).
# Контекст сборки — этот каталог: docker build -f wrapper.Dockerfile --target wrapper .
# region wrapper
FROM messenger:local AS app

# busybox — ради sh: в distroless его нет. ENTRYPOINT — exec-форма, но запускает
# скрипт, а скрипт запускает messenger без exec.
FROM busybox:1.37@sha256:bdf57e528e45e4433820e045b29b4597825a1c9e38353532d90a01445013f82e AS wrapper
COPY --from=app /messenger /messenger
COPY --chmod=0755 start.sh /start.sh
ENV MESSENGER_ADDR=:8080
USER 65532:65532
ENTRYPOINT ["/start.sh"]
# endregion wrapper

# region shell-form
# Shell-форма: Docker запишет ENTRYPOINT как ["/bin/sh", "-c", "/messenger"].
FROM wrapper AS shell-form
ENTRYPOINT /messenger
# endregion shell-form
