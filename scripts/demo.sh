#!/usr/bin/env bash
# ============================================================
# ZK Age Verification — Integration Demo (O5 / O6)
#
# Prerequisites:
#   1. go run zk/setup/main.go                             (Z2 — keygen)
#   2. cp zk/keys/vk_embedded.go chain/x/ageverify/keeper/ (C1 — real VK)
#   3. cd chain && ignite chain build                       (builds ageverifyd)
#   4. ignite chain serve   (in a separate terminal)
#
# Run from repo root:
#   bash scripts/demo.sh
# ============================================================

set -euo pipefail

CHAIN_ID="ageverify"
NODE="tcp://localhost:26657"
KEYRING="--keyring-backend test"
PROOF_FILE="/tmp/zk_age_proof.json"

# ---- colour helpers ----
GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; NC='\033[0m'
step() { echo -e "\n${YELLOW}==> $*${NC}"; }
ok()   { echo -e "${GREEN}✓  $*${NC}"; }
fail() { echo -e "${RED}✗  $*${NC}"; exit 1; }

# ============================================================
# 0. Prerequisites
# ============================================================
step "Checking prerequisites"

command -v ageverifyd >/dev/null 2>&1 \
    || fail "ageverifyd not in PATH. Build with: cd chain && ignite chain build"
command -v jq >/dev/null 2>&1 \
    || fail "jq not found. Install with: apt install jq"
curl -sf "http://localhost:26657/status" >/dev/null 2>&1 \
    || fail "Chain not running at $NODE. Start with: ageverifyd start"

ok "ageverifyd found, chain is live"

# ============================================================
# 1. Generate ZK proof (person born 2000-06-15 → adult)
# ============================================================
step "Generating ZK proof for birthdate 2000-06-15"

go run zk/prover/main.go --year 2000 --month 6 --day 15 > "$PROOF_FILE"

PROOF=$(jq -r .proof           "$PROOF_FILE")
WITNESS=$(jq -r .public_witness "$PROOF_FILE")
DATE=$(jq -r .current_date     "$PROOF_FILE")

echo "  current_date : $DATE"
echo "  proof (first 50): ${PROOF:0:50}..."
ok "Proof generated → $PROOF_FILE"

# ============================================================
# 2. Submit valid proof to chain
# ============================================================
step "Submitting proof to chain (from alice)"

ALICE_ADDR=$(ageverifyd keys show alice -a $KEYRING)
echo "  alice: $ALICE_ADDR"

TX_OUT=$(ageverifyd tx ageverify submit-age-proof \
    "$PROOF" "$WITNESS" "$DATE" \
    --from alice $KEYRING \
    --chain-id "$CHAIN_ID" \
    --node "$NODE" \
    --broadcast-mode sync \
    --yes \
    --output json 2>&1)

TX_HASH=$(echo "$TX_OUT" | jq -r '.txhash // "unknown"')
TX_CODE=$(echo "$TX_OUT" | jq -r '.code  // 0')

echo "  txhash: $TX_HASH"
[ "$TX_CODE" = "0" ] || fail "Broadcast failed (code $TX_CODE): $TX_OUT"
ok "Tx broadcast accepted"

# wait for block inclusion
echo "  Waiting for block inclusion..."
sleep 4

# confirm on-chain via raw CometBFT RPC (avoids cosmos.tx.v1beta1.Tx codec issue)
CONFIRM=$(curl -sf "http://localhost:26657/tx?hash=0x${TX_HASH}" 2>/dev/null || echo '{}')
CONFIRM_CODE=$(echo "$CONFIRM" | jq -r '.result.tx_result.code // 99')
[ "$CONFIRM_CODE" = "0" ] || fail "Tx failed on-chain (code $CONFIRM_CODE): $(echo "$CONFIRM" | jq -r '.result.tx_result.log // ""')"
ok "Tx included in block"

# ============================================================
# 3. Query verification status
# ============================================================
step "Querying verification status for alice"

STATUS=$(ageverifyd query ageverify verification-status "$ALICE_ADDR" \
    --node "$NODE" --output json 2>&1)

VERIFIED=$(echo "$STATUS"    | jq -r '.verified    // "false"')
VERIFIED_AT=$(echo "$STATUS" | jq -r '.verified_at // ""')

echo "  verified    : $VERIFIED"
echo "  verified_at : $VERIFIED_AT"
[ "$VERIFIED" = "true" ] || fail "Expected verified=true, got: $VERIFIED"
ok "Alice is age-verified on-chain ✓"

# ============================================================
# 4. Rejection test — invalid proof bytes
# ============================================================
step "Testing rejection of a fake proof (from bob)"

BOB_ADDR=$(ageverifyd keys show bob -a $KEYRING 2>/dev/null || echo "")
SENDER="alice"
[ -n "$BOB_ADDR" ] && SENDER="bob"

FAKE=$(echo "this-is-not-a-valid-proof"   | base64)
FAKE_WIT=$(echo "this-is-not-a-valid-witness" | base64)

REJECT_OUT=$(ageverifyd tx ageverify submit-age-proof \
    "$FAKE" "$FAKE_WIT" "$DATE" \
    --from "$SENDER" $KEYRING \
    --chain-id "$CHAIN_ID" \
    --node "$NODE" \
    --broadcast-mode sync \
    --yes \
    --output json 2>&1) || true   # don't abort on non-zero exit

REJECT_HASH=$(echo "$REJECT_OUT" | jq -r '.txhash // ""')

# Wait for on-chain execution result (sync mode only confirms mempool acceptance)
REJECT_CODE=0
if [ -n "$REJECT_HASH" ]; then
    sleep 4
    REJECT_RESULT=$(curl -sf "http://localhost:26657/tx?hash=0x${REJECT_HASH}" 2>/dev/null || echo '{}')
    REJECT_CODE=$(echo "$REJECT_RESULT" | jq -r '.result.tx_result.code // 0')
fi

echo "  rejection code: $REJECT_CODE (expected non-zero)"
[ "$REJECT_CODE" != "0" ] \
    || fail "Chain accepted invalid proof — something is wrong with the verifier!"
ok "Chain correctly rejected fake proof (code $REJECT_CODE)"

# ============================================================
# 5. Rejection test — minor (age 15)
# ============================================================
step "Testing that minor cannot generate proof (local check)"

go run zk/prover/main.go --year 2011 --month 1 --day 1 > /dev/null 2>&1 \
    && fail "Prover should have failed for a minor but it succeeded!" \
    || ok "Prover correctly refused to generate proof for minor"

# ============================================================
# Summary
# ============================================================
echo ""
echo "╔══════════════════════════════════════════════╗"
echo "║          DEMO COMPLETE — ALL CHECKS PASSED   ║"
echo "║                                              ║"
echo "║  ✓ ZK proof generated locally (birth date   ║"
echo "║    never left the machine)                   ║"
echo "║  ✓ Valid proof accepted by chain             ║"
echo "║  ✓ Address verified in on-chain state        ║"
echo "║  ✓ Fake proof rejected with error code       ║"
echo "║  ✓ Minor cannot produce a valid proof        ║"
echo "╚══════════════════════════════════════════════╝"
