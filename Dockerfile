FROM golang:1.26-bookworm

RUN apt-get update && apt-get install -y jq curl git python3 && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace
