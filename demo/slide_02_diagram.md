# How it works — data flow

```mermaid
flowchart TB
    subgraph OFF["🔒 Off-chain — user's device (Bob)"]
        DOB["Date of birth<br/>PRIVATE — never leaves the device"]
        PROVER["Prover<br/>gnark / Groth16"]
        PROOF["ZK proof + public inputs<br/>(current date, MinAge)"]
        DOB --> PROVER --> PROOF
    end

    subgraph CHAIN["⛓️ On-chain — blockchain (validator: Alice)"]
        CONS["CometBFT<br/>consensus · ordering · block"]
        MS["Rebuild witness from TRUSTED values<br/>MinAge = 18 + block time"]
        VERIFY{"Groth16.Verify<br/>embedded verification key"}
        STORE["World state:<br/>verified = true"]
        REJECT["Rejected<br/>state unchanged"]
        CONS --> MS --> VERIFY
        VERIFY -->|valid| STORE
        VERIFY -->|invalid| REJECT
    end

    PROOF -->|"broadcast tx"| CONS
    STORE --> Q["Query:<br/>is this address verified?"]
```

**Read it left to right:**

1. The **date of birth stays in the left box** — only the proof crosses the trust boundary.
2. **CometBFT** orders the transaction and puts it in a block (consensus).
3. The chain **rebuilds the public inputs itself** from trusted values — this is the security fix.
4. **Groth16.Verify** runs inside the state machine; valid → stored, invalid → rejected.
5. Anyone can **query** the result; the birth date was never revealed.
