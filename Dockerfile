FROM golang:1.22-bookworm

RUN apt-get update && apt-get install -y jq curl git && rm -rf /var/lib/apt/lists/*

# Install Ignite CLI v28 (latest patch)
RUN curl https://get.ignite.com/cli@v28 | bash

# Pre-warm gnark compilation cache so first run is fast
RUN mkdir /tmp/gnark-warmup && cd /tmp/gnark-warmup && \
    go mod init warmup && \
    go get github.com/consensys/gnark@v0.14.0 && \
    go get github.com/consensys/gnark-crypto@v0.20.0 && \
    go build github.com/consensys/gnark/... && \
    rm -rf /tmp/gnark-warmup

WORKDIR /workspace
