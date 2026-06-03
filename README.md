# ZK Age Verification on Custom Blockchain

Proves age ≥ 18 on-chain without revealing date of birth.
Uses Groth16 zero-knowledge proofs (gnark) on a Cosmos SDK v0.50 chain.

## Components

| Component | Location | Role |
|-----------|----------|------|
| gnark circuit | `zk/circuit/` | Defines age-check computation as constraints |
| CLI prover | `zk/prover/` | Takes birth date, produces ZK proof locally |
| Chain module | `chain/x/ageverify/` | Verifies proof on-chain, stores result |

## Prerequisites

**Docker and Docker Compose only** — no local Go installation needed.

```bash
docker compose build   # ~5 min first time (downloads Go 1.26, compiles chain)
```

## Quick Start (Docker)

All commands run inside the container. Start it once and leave it running:

```bash
# Start container in background
docker compose up -d

# Enter a shell inside it
docker compose exec dev bash
```

Then inside the container:

```bash
cd /workspace

# 1. Initialise a fresh chain (creates keys, genesis, config)
bash scripts/init-chain.sh

# 2. Start the node (in background or a second terminal)
ageverifyd start --minimum-gas-prices 0stake &

# 3. Run the full end-to-end demo
bash scripts/demo.sh
```

The demo takes about 10 seconds and runs five checks (see below).

## What the Demo Does

1. **Generates a ZK proof** for birthdate 2000-06-15 (age 25) — birth date never leaves the machine
2. **Submits the proof** to the chain from alice's account
3. **Queries on-chain state** → `verified: true` for alice's address
4. **Sends a fake proof** → chain rejects it (error code 1103)
5. **Attempts proof for a minor** (born 2011) → local prover refuses

Expected output:

```
✓  ageverifyd found, chain is live
✓  Proof generated → /tmp/zk_age_proof.json
✓  Tx broadcast accepted
✓  Tx included in block
✓  Alice is age-verified on-chain ✓
✓  Chain correctly rejected fake proof (code 1103)
✓  Prover correctly refused to generate proof for minor

╔══════════════════════════════════════════════╗
║          DEMO COMPLETE — ALL CHECKS PASSED   ║
╚══════════════════════════════════════════════╝
```

## Manual Usage (inside container)

```bash
# Generate proof locally (birth date never leaves the machine)
cd /workspace
go run zk/prover/main.go --year 2000 --month 6 --day 15 > /tmp/proof.json

# Optionally require a different minimum age (default is 18)
go run zk/prover/main.go --year 2000 --month 6 --day 15 --min-age 21 > /tmp/proof.json

# Submit to chain
PROOF=$(jq -r .proof   /tmp/proof.json)
WITNESS=$(jq -r .public_witness /tmp/proof.json)
DATE=$(jq -r .current_date /tmp/proof.json)

ageverifyd tx ageverify submit-age-proof "$PROOF" "$WITNESS" "$DATE" \
    --from alice --keyring-backend test \
    --chain-id ageverify \
    --node tcp://localhost:26657 \
    --broadcast-mode sync \
    --yes

# Query verification status
ageverifyd query ageverify verification-status \
    $(ageverifyd keys show alice -a --keyring-backend test) \
    --node tcp://localhost:26657 --output json
```

## Re-running from Scratch

`init-chain.sh` wipes and re-creates all chain state, so you can run it again
at any time:

```bash
pkill ageverifyd 2>/dev/null   # stop any running node
bash scripts/init-chain.sh     # fresh genesis
ageverifyd start --minimum-gas-prices 0stake &
bash scripts/demo.sh
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.26 |
| ZK proofs | gnark v0.14.0 — Groth16, BN254 curve |
| Chain framework | Cosmos SDK v0.50.10 |
| Consensus | CometBFT v0.38 |
| Storage | LevelDB (via cosmos-db v1.0.2) |
| Build environment | Docker (golang:1.26-bookworm) |

## Project Structure

```
zk-age-verify/
├── Dockerfile / docker-compose.yml   # pinned build environment
├── go.work                           # Go workspace (chain + zk modules)
├── scripts/
│   ├── init-chain.sh                 # bootstrap genesis + accounts
│   └── demo.sh                       # end-to-end demo (5 checks)
├── chain/                            # Cosmos SDK chain
│   ├── app/app.go                    # application wiring
│   ├── cmd/ageverifyd/               # node binary entrypoint
│   ├── proto/                        # protobuf definitions + gen.sh
│   │   └── ageverify/ageverify/v1/   # tx.proto, query.proto
│   └── x/ageverify/                  # custom module
│       ├── keeper/                   # state + message handlers
│       ├── types/                    # generated pb.go + helpers
│       ├── module/                   # SDK module registration
│       └── client/cli/               # CLI commands
└── zk/                               # ZK tooling
    ├── circuit/age_circuit.go        # gnark circuit definition
    ├── setup/main.go                 # trusted setup (keygen)
    ├── prover/main.go                # CLI proof generator
    └── keys/                         # generated keys (gitignored)
```

## Notes

- The verification key (`chain/x/ageverify/keeper/vk_embedded.go`) is
  pre-generated and committed. To regenerate it from scratch:
  ```bash
  go run zk/setup/main.go
  cp zk/keys/vk_embedded.go chain/x/ageverify/keeper/
  cd chain && go build -o /usr/local/bin/ageverifyd ./cmd/ageverifyd/
  ```

- The `--minimum-gas-prices 0stake` flag is required when starting the node;
  zero fees are fine for local development.

- The module's protobuf types (`tx.pb.go`, `query.pb.go`) are generated from
  the `.proto` files in `chain/proto/`. To regenerate after editing a proto:
  ```bash
  bash chain/proto/gen.sh
  ```

- Tx confirmation uses the raw CometBFT RPC endpoint
  (`/tx?hash=0x<hash>`) rather than `ageverifyd query tx` to keep the demo
  independent of the SDK's tx-decoding codec.
