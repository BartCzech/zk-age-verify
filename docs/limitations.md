# Limitations & Security Considerations

This document covers known weaknesses of the current implementation and how they would be addressed in a production system.

---

## 1. No Trusted Issuer — User Can Lie About Birth Date

**What:** The circuit proves "given this birth date, the person is ≥ 18." It does not prove the birth date is correct. A user can input any birth date they want.

**Why it matters:** The ZK proof guarantees mathematical correctness of the age calculation, not the truthfulness of the input. A 15-year-old can prove age ≥ 18 by entering a fake birth year.

**Production mitigation:** Bind the birth date to a trusted document. The circuit would include a signature check: the prover must know a birth date signed by a trusted authority (e.g., a government eID using EdDSA). The authority's public key becomes a public input verified on-chain. This adds ~1000–5000 constraints for the signature check but closes the lying-about-age loophole entirely.

---

## 2. Single-Party Trusted Setup — Forged Proofs Possible

**What:** `go run zk/setup/main.go` runs a local Groth16 trusted setup. The person running it knows the "toxic waste" (random trapdoor values used during setup).

**Why it matters:** Anyone who knows the toxic waste can forge a valid proof for any statement, including "I am 18" without a valid birth date. This breaks the entire security guarantee.

**Production mitigation:**
- **Multi-Party Computation (MPC) ceremony**: multiple independent parties each contribute randomness. As long as at least one participant is honest and destroys their share, the setup is secure. Zcash and Ethereum's KZG ceremony used this approach.
- **PLONK / STARKs**: these proof systems use a universal, reusable setup (PLONK) or no trusted setup at all (STARKs). gnark supports PLONK.

---

## 3. Replay Attacks — Proof Reusable From Another Address

**What:** The proof does not bind the prover's blockchain address. Anyone who intercepts `proof.json` can submit it from their own address and get verified.

**Why it matters:** Verification is supposed to prove that *this address's owner* is ≥ 18. Currently it only proves that *someone with the birth date* is ≥ 18. The proof is portable.

**Production mitigation:** Include the submitter's address as a public input to the circuit. The prover commits to their address inside the proof. The chain verifies that the public input address matches `msg.Creator`. Alternatively, add a nullifier derived from the birth date and address to prevent the same birth date being used twice.

---

## 4. Block Time Reliance — Approximate Date Validation

**What:** The chain validates that the proof's `current_date` is within 24 hours of `ctx.BlockTime()`. Block time is set by validators and can drift.

**Why it matters:** Validators could theoretically manipulate block time by up to the consensus-allowed drift. A malicious validator majority could accept a proof with a future date (e.g., to make a 17-year-old appear 18 next month). In practice, CometBFT limits block time drift, but the 24-hour tolerance is generous.

**Production mitigation:** Tighten the tolerance to ±1 hour. Use a time oracle (e.g., Chainlink or IBC-relayed timestamps) for a trustless external time source. Alternatively, pass block height as the public input instead of date, and derive the date from block height via a known genesis block timestamp.

---

## 5. No Revocation — Verified Forever

**What:** Once an address is marked `verified: true`, there is no mechanism to revoke it. The record stays in state indefinitely.

**Why it matters:** If a verified account is compromised, or if the trusted setup is later revealed to have been compromised, there is no way to invalidate existing verifications.

**Production mitigation:**
- **Expiration**: store the verification with an expiry block height (e.g., `verified_until: 10000000`). Queries return `verified: false` if the current block height exceeds the expiry.
- **Revocation list**: maintain an admin-controlled set of revoked addresses.
- **Re-verification requirement**: require re-submission every N blocks to keep verification active.

---

## 6. Approximate Range Checks on Private Inputs

**What:** The circuit uses bit-decomposition for range checks, which are approximate:
- Month: `(BirthMonth - 1)` must fit in 4 bits → allows 1–16, not 1–12
- Day: `(BirthDay - 1)` must fit in 5 bits → allows 1–32, not 1–31
- Year: `(BirthYear - 1900)` must fit in 8 bits → allows 1900–2155

A prover could use month=13 or day=32 without the circuit rejecting it.

**Why it matters:** Invalid dates produce incorrect MMDD values in the birthday-passed comparison. Month=13, day=32 → MMDD=1332, which is above December 31 (1231), so the birthday is always considered "not yet passed." This slightly shifts age calculations for invalid inputs — but only the prover is affected (they're trying to cheat themselves).

**Production mitigation:** Add exact range checks using lookup tables (Plookup/logUp) or additional arithmetic constraints. For month, assert `month - 1 ≤ 11` (fits in 4 bits AND ≤ 11). These cost additional constraints but eliminate the approximation. For this project, the approximate checks are documented and acceptable.

---

## Summary Table

| # | Limitation | Impact | Production Fix |
|---|-----------|--------|----------------|
| 1 | No trusted issuer | User can lie about birth date | EdDSA signature from eID in circuit |
| 2 | Single-party setup | Toxic waste holder can forge proofs | MPC ceremony or PLONK |
| 3 | No address binding | Proof portable to any address | Address as public circuit input |
| 4 | Block time tolerance ±24h | Approximate date validation | Tighter tolerance + time oracle |
| 5 | No revocation | Verified forever | Expiry block height in state |
| 6 | Approximate range checks | Month allows 1–16, day 1–32 | Lookup-table exact range checks |
