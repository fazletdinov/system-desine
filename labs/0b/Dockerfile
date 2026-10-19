FROM golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e

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
