FROM golang:1.27.3-trixie@sha256:fbb7ae40b62ff8afe31662df570d39b4f5052ac32b90dae8e60d7657a20a74f4

RUN apt-get update \
 && apt-get install -y --no-install-recommends tcpdump iproute2 iputils-ping curl dnsutils openssl procps strace \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod ./
COPY 0b ./0b
# Собираем все программы лабораторной.
RUN set -e; for d in 0b/cmd/*/; do [ -d "$d" ] || continue; go build -o /usr/local/bin/$(basename "$d") ./"$d"; done

WORKDIR /lab
CMD ["sleep", "infinity"]
