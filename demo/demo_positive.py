#!/usr/bin/env python3
"""
POSITIVE demo — an adult (age >= 18) gets verified on-chain.

Walks the full lifecycle. The spoken narration for each step lives in
docs/skrypt_03_demo.md (Polish); this script only prints the commands,
their raw output, and the key facts (c.info) so the screen stays clean.

Run inside the container, from the repo root:
    python3 demo/demo_positive.py
"""

import chainlib as c

# ====================  TWEAK THESE  ====================
BIRTH_YEAR  = 2000      # an adult
BIRTH_MONTH = 6
BIRTH_DAY   = 15
ACCOUNT     = "bob"     # an ordinary user, NOT the validator (that's alice)
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
    c.ok("Node is live and producing blocks")

    # ---- STEP 2: accounts are public-key identities ----
    c.step(2, "Identify the account")
    addr = c.address_of(ACCOUNT)
    c.info(ACCOUNT, addr)
    c.ok(f"Acting as user '{ACCOUNT}'")

    # ---- STEP 2b: world state BEFORE (the 'before' half of the diff) ----
    c.step("2b", "Inspect world state BEFORE")
    before = c.query_status(addr)
    c.info("verified (before)", before.get("verified"))
    if before.get("verified") in (True, "true"):
        c.fail(f"'{ACCOUNT}' is already verified — run scripts/init-chain.sh for a clean state")
    c.ok(f"'{ACCOUNT}' is NOT yet verified on-chain")

    # ---- STEP 3: the secret never leaves the machine ----
    c.step(3, "Generate the ZK proof locally (off-chain)")
    c.info("birth date", f"{BIRTH_YEAR}-{BIRTH_MONTH:02d}-{BIRTH_DAY:02d}  (PRIVATE)")
    proof = c.generate_proof(BIRTH_YEAR, BIRTH_MONTH, BIRTH_DAY)
    c.info("proof", proof["proof"][:48] + "…")
    c.info("current_date", proof["current_date"])
    c.ok("Proof generated — date of birth stayed local")

    # ---- STEP 4: a transaction is a signed message ----
    c.step(4, "Sign & broadcast a transaction")
    bcast = c.submit_proof(ACCOUNT, proof)
    txhash = bcast.get("txhash", "")
    if str(bcast.get("code", 0)) != "0":
        c.fail(f"mempool rejected the tx: {bcast.get('raw_log')}")
    c.info("tx hash", txhash)
    c.ok("Transaction accepted into the mempool")

    # ---- STEP 5: mempool -> consensus -> block ----
    c.step(5, "Wait for inclusion in a block")
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
    ev = c.find_event(res, "age_verified")
    if ev:
        c.info("event", "age_verified")
        for k, v in ev.items():
            c.info(f"  {k}", v)
    c.ok("Proof verified on-chain (result code 0)")

    # ---- STEP 7: world state AFTER (the 'after' half of the diff) ----
    c.step(7, "Query the resulting on-chain state AFTER")
    status = c.query_status(addr)
    c.info("verified (before)", before.get("verified"))
    c.info("verified (after)", status.get("verified"))
    c.info("verified_at", status.get("verified_at"))
    if status.get("verified") not in (True, "true"):
        c.fail("expected verified=true in on-chain state")
    c.ok(f"'{ACCOUNT}' is age-verified on-chain")

    c.banner("POSITIVE DEMO COMPLETE — adult verified, privacy preserved")


if __name__ == "__main__":
    main()
