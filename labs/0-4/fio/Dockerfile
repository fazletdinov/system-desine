# Образ с fio для замера SSD: на хосте fio может не быть, а версия в контейнере одна у всех.
# Тег для человека, digest и версия пакета — для воспроизводимости: на них сняты results/.
FROM alpine:3.22@sha256:2e0b81d60597f449baa56dc25d355367869d2dca4cd57e866f1e2b52af3eb55f
RUN apk add --no-cache fio=3.39-r0 jq
COPY run.sh /usr/local/bin/lab-fio
ENTRYPOINT ["/usr/local/bin/lab-fio"]
