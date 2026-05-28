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

- Go 1.22+
- [Ignite CLI v28](https://docs.ignite.com/): `curl https://get.ignite.com/cli@v28 | bash`
- `jq`: `apt install jq`

Or use Docker (pins all versions):
```bash
docker compose build
docker compose run --rm dev bash
```

## Setup

```bash
# 1. Clone and enter container (or use local env)
git clone <repo>
cd zk-age-verify

# 2. Install Go dependencies
cd zk && go mod tidy && cd ..
cd chain && go mod tidy && cd ..

# 3. Run circuit tests (~2-3 min first time)
go test ./zk/circuit/ -v -timeout 10m

# 4. Generate keys (trusted setup)
go run zk/setup/main.go
# Output: zk/keys/proving.key, verification.key, vk_embedded.go

# 5. Copy verification key into chain module
cp zk/keys/vk_embedded.go chain/x/ageverify/keeper/

# 6. Build the chain
cd chain && ignite chain build
```

## Running the Demo

```bash
# Terminal 1 — start the chain
ignite chain serve

# Terminal 2 — run the full demo
bash scripts/demo.sh
```

The demo:
1. Generates a ZK proof for birthdate 2000-06-15 (age 25)
2. Submits the proof to the chain
3. Queries verification status → `verified: true`
4. Sends a fake proof → chain rejects it
5. Attempts proof generation for a minor → local prover fails

## Manual Usage

```bash
# Generate proof (birth date stays local)
go run zk/prover/main.go --year 2000 --month 6 --day 15 > proof.json

# Sanity-check proof before sending to chain
go run zk/verify/main.go < proof.json

# Submit to chain
PROOF=$(jq -r .proof proof.json)
WITNESS=$(jq -r .public_witness proof.json)
DATE=$(jq -r .current_date proof.json)
ageverifyd tx ageverify submit-age-proof "$PROOF" "$WITNESS" "$DATE" \
    --from alice --keyring-backend test --chain-id ageverify -y

# Query result
ageverifyd query ageverify verification-status $(ageverifyd keys show alice -a --keyring-backend test)
```

## Tech Stack

- **Go 1.22** — single language end-to-end
- **Cosmos SDK v0.50.10** — chain framework
- **CometBFT v0.38** — consensus
- **gnark v0.14.0** — ZK proof system (Groth16, BN254 curve)
- **gnark-crypto v0.20.0** — elliptic curve arithmetic

## Project Structure

```
zk-age-verify/
├── Dockerfile / docker-compose.yml   # pinned build environment
├── go.work                           # Go workspace (chain + zk modules)
├── scripts/demo.sh                   # end-to-end demo
├── chain/                            # Cosmos SDK chain
│   ├── app/app.go                    # application wiring
│   ├── cmd/ageverifyd/               # node binary entrypoint
│   └── x/ageverify/                  # custom module
│       ├── keeper/                   # state + message handlers
│       ├── types/                    # message and query types
│       ├── module/                   # SDK module registration
│       └── client/cli/               # CLI commands
└── zk/                               # ZK tooling
    ├── circuit/age_circuit.go        # gnark circuit definition
    ├── setup/main.go                 # trusted setup (keygen)
    ├── prover/main.go                # CLI proof generator
    ├── verify/main.go                # standalone verifier (debugging)
    └── keys/                         # generated keys (gitignored)
```
