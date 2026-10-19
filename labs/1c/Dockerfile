# syntax=docker/dockerfile:1.19@sha256:b6afd42430b15f2d2a4c5a02b919e98a525b785b1aaff16747d2f623364e39b6

# region rootfs
# Rootfs контейнера — файловая система образа busybox, закреплённого по digest.
# Это ровно то, что docker run распаковал бы для busybox: каталоги и статический бинарь.
FROM busybox:1.37@sha256:bdf57e528e45e4433820e045b29b4597825a1c9e38353532d90a01445013f82e AS rootfs
# endregion rootfs

FROM golang:1.27.0-trixie@sha256:df98008ecd2b0ecc9f0a94d1b07e3564a9c92b555369b33d9b5f60d0765b2db7 AS build
WORKDIR /src
COPY go.mod ./
COPY 1c ./1c
# hog — статический: в rootfs нет libc.
RUN CGO_ENABLED=0 go build -o /out/minicontainer ./1c/cmd/minicontainer \
 && CGO_ENABLED=0 go build -o /out/hog ./1c/cmd/hog

FROM golang:1.27.0-trixie@sha256:df98008ecd2b0ecc9f0a94d1b07e3564a9c92b555369b33d9b5f60d0765b2db7
RUN apt-get update \
 && apt-get install -y --no-install-recommends iproute2 procps util-linux \
 && rm -rf /var/lib/apt/lists/*
COPY --from=rootfs / /rootfs
COPY --from=build /out/hog /rootfs/bin/hog
COPY --from=build /out/minicontainer /usr/local/bin/minicontainer
COPY 1c/entrypoint.sh 1c/scenario.sh /lab/
WORKDIR /lab
ENTRYPOINT ["/lab/entrypoint.sh"]
CMD ["bash"]
