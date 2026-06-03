#!/usr/bin/env bash
# Regenerate Go code from proto files.
# Run from repo root inside the dev container:
#   bash chain/proto/gen.sh
#
# Prerequisites (already satisfied in the dev container):
#   apt-get install -y protobuf-compiler
#   cd /go/pkg/mod/github.com/cosmos/gogoproto@v1.7.0/protoc-gen-gocosmos
#   go build -o /usr/local/bin/protoc-gen-gocosmos .

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CHAIN_DIR="$(dirname "$SCRIPT_DIR")"
SDK_PROTO="/go/pkg/mod/github.com/cosmos/cosmos-sdk@v0.50.10/proto"
GOGOPROTO="/go/pkg/mod/github.com/cosmos/gogoproto@v1.7.0"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

protoc \
  -I "$SCRIPT_DIR" \
  -I "$SDK_PROTO" \
  -I "$GOGOPROTO" \
  --gocosmos_out=plugins=grpc:"$tmp" \
  ageverify/ageverify/v1/tx.proto \
  ageverify/ageverify/v1/query.proto

cp "$tmp"/ageverify/x/ageverify/types/*.pb.go \
   "$CHAIN_DIR/x/ageverify/types/"

echo "Done — generated files copied to chain/x/ageverify/types/"
