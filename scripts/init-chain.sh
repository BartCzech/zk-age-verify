#!/usr/bin/env bash
# ============================================================
# init-chain.sh — bootstrap a single-validator ageverify chain
#
# Run this ONCE (from inside the Docker container, from /workspace):
#   bash scripts/init-chain.sh
#
# Then start the node in a separate terminal (or background it):
#   ageverifyd start
# ============================================================

set -euo pipefail

CHAIN_ID="ageverify"
MONIKER="validator"
HOME_DIR="$HOME/.ageverify"
KEYRING="--keyring-backend test"

echo "==> Wiping any previous chain state"
rm -rf "$HOME_DIR"

echo "==> Initialising chain (home: $HOME_DIR)"
ageverifyd init "$MONIKER" --chain-id "$CHAIN_ID"

echo "==> Creating test accounts (alice = validator, bob = adult user, charlie = minor)"
ageverifyd keys add alice   $KEYRING
ageverifyd keys add bob     $KEYRING
ageverifyd keys add charlie $KEYRING

ALICE=$(ageverifyd keys show alice   -a $KEYRING)
BOB=$(ageverifyd keys show bob     -a $KEYRING)
CHARLIE=$(ageverifyd keys show charlie -a $KEYRING)
echo "    alice (validator): $ALICE"
echo "    bob   (adult user): $BOB"
echo "    charlie (minor):    $CHARLIE"

echo "==> Funding genesis accounts"
ageverifyd genesis add-genesis-account "$ALICE"   100000000stake $KEYRING
ageverifyd genesis add-genesis-account "$BOB"     100000000stake $KEYRING
ageverifyd genesis add-genesis-account "$CHARLIE" 100000000stake $KEYRING

echo "==> Creating genesis validator transaction (alice self-delegates 10 000 000 stake)"
ageverifyd genesis gentx alice 10000000stake \
    --chain-id "$CHAIN_ID" \
    --moniker "$MONIKER" \
    $KEYRING

echo "==> Collecting genesis transactions"
ageverifyd genesis collect-gentxs

echo "==> Validating genesis"
ageverifyd genesis validate "$HOME_DIR/config/genesis.json"

echo ""
echo "Done. Start the node with:"
echo "  ageverifyd start --minimum-gas-prices 0stake"
