# TODO: Remove me before merging PR. This is for testing purposes only

FROM ubuntu:rolling AS builder

ARG REDIS_VERSION=7.0.5

WORKDIR /build

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install wget build-essential libssl-dev tar gzip pkg-config -y
RUN wget --quiet https://github.com/redis/redis/archive/refs/tags/${REDIS_VERSION}.tar.gz -O redis.tar.gz && \
    tar xzf redis.tar.gz && cd redis-${REDIS_VERSION} && make BUILD_TLS=yes

FROM ubuntu:rolling
ARG REDIS_VERSION=7.0.5

WORKDIR /redis

COPY --from=builder /build/redis-${REDIS_VERSION}/src/redis-cli .
COPY --from=builder /build/redis-${REDIS_VERSION}/src/redis-server .

RUN ls -al

ENV PATH="${PATH}:/redis"

# docker run -it --rm -v $(pwd)/tls:/redis/tls -p6379:6379 -p6380:6380 my-redis redis-server --requirepass replicated --tls-port 6380 --port 6379 --tls-cert-file tls/server.pem --tls-key-file tls/server-key.pem --tls-ca-cert-file tls/ca.pem --loglevel debug
