FROM golang:1.27.3-trixie@sha256:af1bfc2614ebb1dbd31af514b78740fd678850e76498762b425ecee4c5c6ea80

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
