# ZK Age Verification

### Prove you are 18+ without revealing your date of birth

---

**The problem.** To prove you are an adult today, you show an ID. That means hand over
your full identity, photo and exact birth date. The verifier only needed *one thing*: 
are you at least 18? Everything else is leaked.

**The idea.** The user generates a **zero-knowledge proof** of the statement
*"my age ≥ 18"* and nothing else. The date of birth is a **private input**: it
never leaves the user's device. Only the proof and the public inputs (today's
date and the age threshold) are shared.

**The result is recorded on a blockchain** - a shared, tamper-proof, trustless
registry of *"this address is age-verified"*. No central authority is trusted
and the answer is permanent and publicly checkable.

---

### Tech stack

| Layer | What we use |
| --- | --- |
| Zero-knowledge | `gnark` · Groth16 · BN254 curve |
| Blockchain | custom **Cosmos SDK** module + **CometBFT** consensus |
| Trust model | verification runs **inside the state machine** — every validator re-checks the proof |

### The key property

> The chain does **not** trust the user's public inputs. It rebuilds them from
> trusted values (MinAge = 18, date from block time) before verifying so a
> mathematically valid proof of "age ≥ 0" or "age ≥ 18 in the year 2200" is
> rejected.
