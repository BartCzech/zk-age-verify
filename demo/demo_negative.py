#!/usr/bin/env python3
"""
NEGATIVE demo — a minor (age < 18) cannot get verified, even by cheating.

Shows N1..N4. The spoken narration for each step lives in
docs/skrypt_03_demo.md (Polish); this script only prints the commands,
their raw output, and the key facts (c.info) so the screen stays clean.

Run inside the container, from the repo root:
    python3 demo/demo_negative.py
"""

import datetime

import chainlib as c

# ====================  TWEAK THESE  ====================
BIRTH_YEAR  = 2015      # a minor (under 18)
BIRTH_MONTH = 1
BIRTH_DAY   = 1
ACCOUNT     = "charlie"  # the minor — a distinct user from the adult 'bob'
ATTACK_MIN_AGE   = 0     # bypass attempt #1: lower the bar to 0
ATTACK_FAKE_YEAR = 2035  # bypass attempt #2: pretend it is the future
# =======================================================


def expect_rejected(label, account, proof_b64, witness_b64, date):
    """Submit a (crafted) proof and assert the chain rejects it on-chain."""
    bcast = c.submit_raw(account, proof_b64, witness_b64, date)
    txhash = bcast.get("txhash", "")
    if not txhash:
        c.fail(f"{label}: no txhash returned ({bcast})")
    c.info("tx hash", txhash)
    res = c.wait_for_tx(txhash)
    if res is None:
        c.fail(f"{label}: tx not committed within timeout")
    code = str(res.get("code", 0))
    c.info("block height", res.get("height"))
    c.info("result code", f"{code}  (non-zero = rejected)")
    c.info("log", res.get("log", "").split(";")[-1].strip())
    if code == "0":
        c.fail(f"{label}: chain ACCEPTED a proof it should have rejected!")
    c.ok(f"{label}: chain rejected the proof (code {code})")
    return code


def main():
    c.banner("ZK AGE VERIFICATION — NEGATIVE CASE (minor)")
    c.require_chain_live()

    addr = c.address_of(ACCOUNT)
    c.info("account", f"{ACCOUNT}  ({addr})")
    c.info("birth date", f"{BIRTH_YEAR}-{BIRTH_MONTH:02d}-{BIRTH_DAY:02d}  (a minor)")

    # ---- N1: ZK cannot prove a false statement ----
    c.step("N1", "Honest attempt — prover should refuse")
    try:
        c.generate_proof(BIRTH_YEAR, BIRTH_MONTH, BIRTH_DAY, min_age=18)
        c.fail("prover produced a proof for a minor — that should be impossible!")
    except c.ProofRefused:
        c.ok("Prover refused — a false statement cannot be proven")

    # ---- N2: forge a weaker statement (MinAge=0) ----
    c.step("N2", f"Attack #1 — forge a proof with MinAge={ATTACK_MIN_AGE}")
    evil = c.generate_proof(BIRTH_YEAR, BIRTH_MONTH, BIRTH_DAY, min_age=ATTACK_MIN_AGE)
    c.ok("Attacker produced a valid MinAge=0 proof locally")
    expect_rejected("Attack #1", ACCOUNT,
                    evil["proof"], evil["public_witness"], evil["current_date"])

    # ---- N3: forge a future "current date" ----
    c.step("N3", f"Attack #2 — forge a proof claiming the year is {ATTACK_FAKE_YEAR}")
    future = c.generate_proof(BIRTH_YEAR, BIRTH_MONTH, BIRTH_DAY,
                              min_age=18, current=(ATTACK_FAKE_YEAR, 1, 1))
    c.ok(f"Attacker produced a valid proof claiming year {ATTACK_FAKE_YEAR}")

    # Vector A: submit with the matching future date (caught by block-time check)
    expect_rejected("Attack #2A", ACCOUNT,
                    future["proof"], future["public_witness"], future["current_date"])

    # Vector B: submit with TODAY's date (chain rebuilds witness from today -> pairing fails)
    today = datetime.date.today().strftime("%Y%m%d")
    expect_rejected("Attack #2B", ACCOUNT,
                    future["proof"], future["public_witness"], today)

    # ---- N4: state is unchanged ----
    c.step("N4", "Confirm on-chain state was never changed")
    status = c.query_status(addr)
    c.info("verified", status.get("verified"))
    if status.get("verified") in (True, "true"):
        c.fail("minor is marked verified — the chain was bypassed!")
    c.ok(f"'{ACCOUNT}' is NOT verified — every bypass was rejected")

    c.banner("NEGATIVE DEMO COMPLETE — minor rejected, chain not bypassed")


if __name__ == "__main__":
    main()
