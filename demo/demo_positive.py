#!/usr/bin/env python3
"""
POSITIVE demo — an adult (age >= 18) gets verified on-chain.

Walks the full lifecycle and points out the blockchain concept at each step:
  1. chain liveness        2. accounts & keys      3. off-chain ZK proof
  4. signed transaction    5. mempool -> block     6. on-chain execution
  7. world-state query

Run inside the container, from the repo root:
    python3 demo/demo_positive.py
"""

import chainlib as c

# ====================  TWEAK THESE  ====================
BIRTH_YEAR  = 2000      # an adult
BIRTH_MONTH = 6
BIRTH_DAY   = 15
ACCOUNT     = "alice"
# =======================================================


def main():
    c.banner("ZK AGE VERIFICATION — POSITIVE CASE (adult)")
    c.require_chain_live()

    # ---- STEP 1: the chain is a live, replicated ledger ----
    c.step(1, "Connect to the blockchain")
    st = c.chain_status()
    c.info("chain id", st["node_info"]["network"])
    c.info("block height", st["sync_info"]["latest_block_height"])
    c.info("block time", st["sync_info"]["latest_block_time"])
    c.note("A blockchain is a live, append-only ledger replicated across nodes,")
    c.note("producing a new block every few seconds.")
    c.ok("Node is live and producing blocks")

    # ---- STEP 2: accounts are public-key identities ----
    c.step(2, "Identify the account")
    addr = c.address_of(ACCOUNT)
    c.info(ACCOUNT, addr)
    c.note("Accounts are identified by a public-key-derived bech32 address.")
    c.note("The keyring holds the private key used to sign transactions.")
    c.ok(f"Acting as '{ACCOUNT}'")

    # ---- STEP 3: the secret never leaves the machine ----
    c.step(3, "Generate the ZK proof locally (off-chain)")
    c.info("birth date", f"{BIRTH_YEAR}-{BIRTH_MONTH:02d}-{BIRTH_DAY:02d}  (PRIVATE)")
    proof = c.generate_proof(BIRTH_YEAR, BIRTH_MONTH, BIRTH_DAY)
    c.info("proof", proof["proof"][:48] + "…")
    c.info("current_date", proof["current_date"])
    c.note("The birth date is a PRIVATE input — it never leaves this machine.")
    c.note("Only the proof + public inputs (current date, MinAge) will go on-chain.")
    c.ok("Proof generated — date of birth stayed local")

    # ---- STEP 4: a transaction is a signed message ----
    c.step(4, "Sign & broadcast a transaction")
    bcast = c.submit_proof(ACCOUNT, proof)
    txhash = bcast.get("txhash", "")
    if str(bcast.get("code", 0)) != "0":
        c.fail(f"mempool rejected the tx: {bcast.get('raw_log')}")
    c.info("tx hash", txhash)
    c.note("The message is signed by the account's key and broadcast to the node.")
    c.ok("Transaction accepted into the mempool")

    # ---- STEP 5: mempool -> consensus -> block ----
    c.step(5, "Wait for inclusion in a block")
    c.note("Mempool acceptance is NOT finality. Validators must include the tx")
    c.note("in a block and agree on it via consensus.")
    res = c.wait_for_tx(txhash)
    if res is None:
        c.fail("tx was not committed within the timeout")
    c.info("block height", res.get("height"))
    c.info("gas used", res.get("gas_used"))
    c.ok("Transaction committed to a block (final)")

    # ---- STEP 6: deterministic on-chain execution ----
    c.step(6, "On-chain ZK verification & event")
    if str(res.get("code", 1)) != "0":
        c.fail(f"on-chain execution failed (code {res.get('code')}): {res.get('log')}")
    c.note("Every validator re-ran the Groth16 verification inside consensus.")
    c.note("Result code 0 = the state transition was applied deterministically.")
    ev = c.find_event(res, "age_verified")
    if ev:
        c.info("event", "age_verified")
        for k, v in ev.items():
            c.info(f"  {k}", v)
    c.ok("Proof verified on-chain (result code 0)")

    # ---- STEP 7: world state ----
    c.step(7, "Query the resulting on-chain state")
    status = c.query_status(addr)
    c.info("verified", status.get("verified"))
    c.info("verified_at", status.get("verified_at"))
    if status.get("verified") not in (True, "true"):
        c.fail("expected verified=true in on-chain state")
    c.note("The verification is now permanent world state — anyone can query it,")
    c.note("yet the birth date was never revealed.")
    c.ok(f"'{ACCOUNT}' is age-verified on-chain")

    c.banner("POSITIVE DEMO COMPLETE — adult verified, privacy preserved")


if __name__ == "__main__":
    main()
