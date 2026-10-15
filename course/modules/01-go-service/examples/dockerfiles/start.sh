#!/bin/sh
# Для урока 1.17, не для продакшена: типичная обёртка, которая готовит окружение и
# запускает сервер без exec. PID 1 становится sh, а messenger — его ребёнком.
set -eu
echo "start.sh: ключ — $MESSENGER_JWT_KEY_FILE" >&2
/messenger
