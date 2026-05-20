# ZK Age Verification — Detailed Task Specs (v2)

> **Changes from v1:**
> - Circuit rewritten: uses `std/math/cmp.IsLess` instead of `api.Cmp` + `IsZero` hack
> - gnark pinned to v0.14.0, gnark-crypto to v0.20.0
> - KV store pattern updated for Cosmos SDK v0.50 (`storeService` not `storeKey`)
> - Dockerfile added (task O0) to pin all versions
> - Pitfalls section added at the end

> **How to use this document:**
> Each coding task is self-contained. To execute:
> 1. Open a fresh AI chat
> 2. Paste the **Project Context** section
> 3. Paste the specific task spec
> 4. AI codes it — you review, understand, commit
>
> Operational tasks (setup, scaffold, integration, demo, docs) are at the end.

---

## Project Context

(Paste this at the top of every new coding chat.)

```
PROJECT: ZK Age Verification on Custom Blockchain
CHAIN NAME: ageverify
LANGUAGE: Go (end-to-end, no exceptions)
STACK:
- Cosmos SDK v0.50.x via Ignite CLI v28.x (latest v28 patch)
- gnark v0.14.0 (ZK proof system, pure Go)
- gnark-crypto v0.20.0
- Groth16 proof system on BN254 elliptic curve

PINNED VERSIONS (both people MUST use identical versions):
- Go: 1.22.x
- Ignite CLI: v28 (latest patch, install via curl https://get.ignite.com/cli@v28 | bash)
- gnark: github.com/consensys/gnark v0.14.0
- gnark-crypto: github.com/consensys/gnark-crypto v0.20.0

WHAT IT DOES:
User proves age >= 18 on-chain without revealing date of birth.
Three components:
1. gnark circuit — defines the age-check computation
2. CLI prover — takes birth date, produces ZK proof locally
3. Cosmos SDK module — verifies proof on-chain, stores result

REPO LAYOUT:
zk-age-verify/
├── Dockerfile                     # Pinned build environment
├── docker-compose.yml
├── chain/                         # Cosmos SDK chain (Ignite-scaffolded)
│   └── x/ageverify/               # Custom module
│       ├── keeper/
│       │   ├── msg_server_submit_age_proof.go
│       │   ├── query_verification_status.go
│       │   ├── verification_store.go
│       │   └── verifier.go
│       └── types/
│           ├── errors.go
│           └── keys.go
├── zk/
│   ├── circuit/
│   │   ├── age_circuit.go
│   │   └── age_circuit_test.go
│   ├── setup/
│   │   └── main.go
│   ├── prover/
│   │   └── main.go
│   └── keys/                      # gitignored, generated locally
├── scripts/
│   └── demo.sh
├── docs/
│   ├── architecture.md
│   └── limitations.md
├── go.work
└── README.md

DESIGN RULES:
- Minimal code. Every line explainable by a student to an examiner.
- No premature abstractions. No "for later" wrappers.
- Comments explain WHY, not WHAT.
- English code, English comments.
```

---

## Role Assignments

| Role | Person | Coding tasks | Operational tasks |
|------|--------|-------------|-------------------|
| **Person A** (Chain) | You | C1 (chain module) | O0 (docker, shared), O1 (setup, shared), O2 (scaffold), O5 (integration, shared), O6 (demo, shared), O7 (architecture docs) |
| **Person B** (ZK) | Colleague | Z1 (circuit), Z2 (keygen), Z3 (CLI prover), Z4 (verify) | O0 (docker, shared), O1 (setup, shared), O5 (integration, shared), O6 (demo, shared), O8 (limitations docs) |

Person B has less total work (~14-16h). Person B should finish Z1–Z3
early and help Person A debug chain integration.

---

# CODING TASKS

---

## Z1 — gnark Circuit: Age Verification

**Assignee:** Person B (~3-4h)

### Context

The circuit is the mathematical core of the project. It defines a computation
that the ZK proof attests to: "I know a birth date (private) such that, given
today's date (public), my age is ≥ 18."

gnark circuits are Go structs. Fields tagged `gnark:",secret"` are private
inputs (only the prover knows them). Fields tagged `gnark:",public"` are
public inputs (the verifier sees them). The `Define(api)` method declares
constraints — mathematical relationships that must hold for a valid proof.

**Circuits are not imperative code.** `Define()` doesn't compute a result —
it declares equations. The prover finds values that satisfy all equations
simultaneously. The verifier checks that a solution exists without seeing the
private values.

### Scope

**In scope:**
- `AgeCircuit` struct with 3 private inputs (birth year/month/day) and 4 public inputs (current year/month/day, min age)
- `Define()` implementing: range checks on inputs, base age calculation, birthday-adjustment logic, age >= minAge assertion
- Thorough unit tests using gnark's `test.NewAssert`

**Out of scope:**
- Key generation (Z2), CLI prover (Z3)
- Trusted issuer / EdDSA signatures
- Leap year handling (not worth the constraint cost)
- Nullifiers, replay protection

### Files to create

```
zk/circuit/age_circuit.go
zk/circuit/age_circuit_test.go
```

### Implementation — `age_circuit.go`

```go
package circuit

import (
	"math/big"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
)

// AgeCircuit proves that a person's age is >= MinAge without revealing
// their date of birth. BirthYear/Month/Day are private (secret) inputs
// known only to the prover. CurrentYear/Month/Day and MinAge are public
// inputs visible to the verifier.
//
// DO NOT REORDER FIELDS. Witness serialization depends on field declaration
// order. If you change the order here, all existing keys and proofs become
// invalid, and the chain verifier will reject everything with a cryptic
// "verification failed" error.
type AgeCircuit struct {
	// Private inputs — never leave the prover's machine
	BirthYear  frontend.Variable `gnark:",secret"`
	BirthMonth frontend.Variable `gnark:",secret"`
	BirthDay   frontend.Variable `gnark:",secret"`

	// Public inputs — the chain (verifier) sees these
	CurrentYear  frontend.Variable `gnark:",public"`
	CurrentMonth frontend.Variable `gnark:",public"`
	CurrentDay   frontend.Variable `gnark:",public"`
	MinAge       frontend.Variable `gnark:",public"`
}

// Define declares the constraints that must hold for a valid proof.
// This method does NOT execute computation — it builds a constraint system.
//
// Logic:
//  1. Validate ranges of private inputs (month 1-12, day 1-31, year 1900-2155)
//  2. Compute base age = currentYear - birthYear
//  3. Determine if birthday has already passed this year
//  4. Adjust age down by 1 if birthday hasn't passed
//  5. Assert adjustedAge >= minAge
func (c *AgeCircuit) Define(api frontend.API) error {

	// ---------------------------------------------------------------
	// STEP 1: Range checks on private inputs.
	//
	// Why: Without these, a malicious prover could submit values like
	// birthMonth=9999 which would produce nonsensical but technically
	// satisfiable constraints. Range checks ensure inputs are realistic.
	//
	// How: api.ToBinary(x, n) decomposes x into n bits. If x doesn't
	// fit in n bits (x >= 2^n), the constraint system is unsatisfiable.
	// In a finite field, "negative" numbers are actually huge (close to
	// p ≈ 2^254), so they don't fit either — this catches underflows.
	//
	// NOTE: These checks are approximate:
	//   - Month allows 1-16 (4 bits), not exactly 1-12
	//   - Day allows 1-32 (5 bits), not exactly 1-31
	//   - Year allows 1900-2155 (8 bits offset from 1900)
	//   Exact checks would cost extra constraints. For this project,
	//   approximate is fine — documented in limitations.md.
	// ---------------------------------------------------------------

	// BirthMonth in [1, 16]: shift to [0, 15], must fit in 4 bits
	api.ToBinary(api.Sub(c.BirthMonth, 1), 4)

	// BirthDay in [1, 32]: shift to [0, 31], must fit in 5 bits
	api.ToBinary(api.Sub(c.BirthDay, 1), 5)

	// BirthYear in [1900, 2155]: shift to [0, 255], must fit in 8 bits
	api.ToBinary(api.Sub(c.BirthYear, 1900), 8)

	// ---------------------------------------------------------------
	// STEP 2: Base age = currentYear - birthYear.
	//
	// This is the age the person WILL turn this year, not necessarily
	// the age they ARE right now (depends on whether birthday passed).
	// ---------------------------------------------------------------

	baseAge := api.Sub(c.CurrentYear, c.BirthYear)

	// ---------------------------------------------------------------
	// STEP 3: Has the birthday passed this calendar year?
	//
	// We encode month+day as a single comparable integer:
	//   MMDD = month * 100 + day
	// Examples: Jan 1 = 101, Dec 31 = 1231, Mar 15 = 315
	//
	// If currentMMDD >= birthMMDD → birthday has passed → age = baseAge
	// If currentMMDD <  birthMMDD → not yet → age = baseAge - 1
	//
	// We use the gnark standard library's cmp.IsLess function:
	//   cmp.IsLess(api, a, b) returns 1 if a < b, 0 if a >= b
	//
	// This is the official gnark API for comparisons — it handles
	// finite field arithmetic correctly internally using bit
	// decomposition. Do NOT use api.Cmp() directly (it returns -1
	// as p-1 in the field, which is error-prone to work with).
	//
	// MMDD values range from 101 to 1231, which fit in 11 bits.
	// For better efficiency, we use BoundedComparator which knows
	// the maximum possible difference (1231 - 101 = 1130).
	// ---------------------------------------------------------------

	currentMMDD := api.Add(api.Mul(c.CurrentMonth, 100), c.CurrentDay)
	birthMMDD := api.Add(api.Mul(c.BirthMonth, 100), c.BirthDay)

	// BoundedComparator is more efficient than unbounded cmp.IsLess
	// when we know the maximum absolute difference between values.
	// Max diff between two MMDD values: 1231 - 101 = 1130.
	// Second arg (false) = deterministic mode (no optimization shortcuts).
	mmddComparator := cmp.NewBoundedComparator(api, big.NewInt(1130), false)

	// birthdayNotPassed = 1 if currentMMDD < birthMMDD (birthday not yet)
	// birthdayNotPassed = 0 if currentMMDD >= birthMMDD (birthday passed or is today)
	birthdayNotPassed := mmddComparator.IsLess(currentMMDD, birthMMDD)

	// ---------------------------------------------------------------
	// STEP 4: Adjusted age.
	//
	// adjustedAge = baseAge - birthdayNotPassed
	//
	// If birthday passed (birthdayNotPassed=0): adjustedAge = baseAge
	// If birthday not passed (birthdayNotPassed=1): adjustedAge = baseAge - 1
	//
	// Circuits have no if/else. We use arithmetic:
	// subtracting 0 or 1 is the branch-free equivalent.
	// ---------------------------------------------------------------

	adjustedAge := api.Sub(baseAge, birthdayNotPassed)

	// ---------------------------------------------------------------
	// STEP 5: Assert adjustedAge >= minAge.
	//
	// Equivalent to: diff = adjustedAge - minAge >= 0.
	//
	// We check this with a bit decomposition: if diff fits in 8 bits
	// (range 0-255), it's non-negative and reasonable.
	//
	// If diff were "negative" (adjustedAge < minAge), then in the
	// finite field diff would be a huge number (~254 bits) that
	// can't fit in 8 bits — ToBinary would fail the constraint.
	//
	// 8 bits allows diff up to 255, meaning max provable age is
	// minAge + 255 = 273 for minAge=18. More than enough.
	// ---------------------------------------------------------------

	diff := api.Sub(adjustedAge, c.MinAge)
	api.ToBinary(diff, 8)

	return nil
}
```

### Implementation — `age_circuit_test.go`

```go
package circuit

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/test"
)

// Helper to build an assignment with common defaults.
func assignment(birthY, birthM, birthD, curY, curM, curD, minAge int) *AgeCircuit {
	return &AgeCircuit{
		BirthYear: birthY, BirthMonth: birthM, BirthDay: birthD,
		CurrentYear: curY, CurrentMonth: curM, CurrentDay: curD,
		MinAge: minAge,
	}
}

// --- HAPPY PATHS (proof should succeed) ---

// Age 25, birthday not yet passed (June 15 > May 13).
// baseAge=26, birthdayNotPassed=1, adjustedAge=25. 25>=18 ✓
func TestAdult_BirthdayNotYetPassed(t *testing.T) {
	assert := test.NewAssert(t)
	assert.ProverSucceeded(
		&AgeCircuit{},
		assignment(2000, 6, 15, 2026, 5, 13, 18),
		test.WithCurves(ecc.BN254),
	)
}

// Age 26, birthday already passed (March 10 < May 13).
// baseAge=26, birthdayNotPassed=0, adjustedAge=26. 26>=18 ✓
func TestAdult_BirthdayAlreadyPassed(t *testing.T) {
	assert := test.NewAssert(t)
	assert.ProverSucceeded(
		&AgeCircuit{},
		assignment(2000, 3, 10, 2026, 5, 13, 18),
		test.WithCurves(ecc.BN254),
	)
}

// Exactly 18, birthday already passed (March 10 < May 13).
// baseAge=18, birthdayNotPassed=0, adjustedAge=18. 18>=18 ✓ (edge case)
func TestExactly18_BirthdayPassed(t *testing.T) {
	assert := test.NewAssert(t)
	assert.ProverSucceeded(
		&AgeCircuit{},
		assignment(2008, 3, 10, 2026, 5, 13, 18),
		test.WithCurves(ecc.BN254),
	)
}

// Exactly 18, birthday is TODAY (May 13 == May 13).
// baseAge=18, birthdayNotPassed=0 (IsLess is strict: 513 < 513 is false),
// adjustedAge=18. 18>=18 ✓
func TestExactly18_BirthdayToday(t *testing.T) {
	assert := test.NewAssert(t)
	assert.ProverSucceeded(
		&AgeCircuit{},
		assignment(2008, 5, 13, 2026, 5, 13, 18),
		test.WithCurves(ecc.BN254),
	)
}

// --- FAILURE PATHS (proof should fail) ---

// Age 17, birthday not passed yet (December 25 > May 13).
// baseAge=18, birthdayNotPassed=1, adjustedAge=17. 17<18 ✗
func TestMinor_17_BirthdayNotPassed(t *testing.T) {
	assert := test.NewAssert(t)
	assert.ProverFailed(
		&AgeCircuit{},
		assignment(2008, 12, 25, 2026, 5, 13, 18),
		test.WithCurves(ecc.BN254),
	)
}

// Age 16. Clearly underage.
// baseAge=16, birthdayNotPassed=0, adjustedAge=16. 16<18 ✗
func TestMinor_16(t *testing.T) {
	assert := test.NewAssert(t)
	assert.ProverFailed(
		&AgeCircuit{},
		assignment(2010, 1, 1, 2026, 5, 13, 18),
		test.WithCurves(ecc.BN254),
	)
}

// --- RANGE CHECK TESTS ---

// BirthMonth=20: (20-1)=19, needs 5 bits (10011), doesn't fit in 4 → fail
func TestInvalidMonth_20(t *testing.T) {
	assert := test.NewAssert(t)
	assert.ProverFailed(
		&AgeCircuit{},
		assignment(2000, 20, 15, 2026, 5, 13, 18),
		test.WithCurves(ecc.BN254),
	)
}

// BirthDay=50: (50-1)=49, needs 6 bits (110001), doesn't fit in 5 → fail
func TestInvalidDay_50(t *testing.T) {
	assert := test.NewAssert(t)
	assert.ProverFailed(
		&AgeCircuit{},
		assignment(2000, 6, 50, 2026, 5, 13, 18),
		test.WithCurves(ecc.BN254),
	)
}

// BirthYear=1800: (1800-1900) is negative → huge field element → doesn't fit in 8 bits → fail
func TestInvalidYear_1800(t *testing.T) {
	assert := test.NewAssert(t)
	assert.ProverFailed(
		&AgeCircuit{},
		assignment(1800, 6, 15, 2026, 5, 13, 18),
		test.WithCurves(ecc.BN254),
	)
}

// --- COMPILATION TEST ---

// Verifies the circuit can be compiled to R1CS without structural errors.
func TestCircuitCompiles(t *testing.T) {
	assert := test.NewAssert(t)
	assert.SolvingSucceeded(
		&AgeCircuit{},
		assignment(2000, 6, 15, 2026, 5, 13, 18),
		test.WithCurves(ecc.BN254),
	)
}
```

### Acceptance criteria

**Circuit struct:**
- `AgeCircuit` has exactly 7 fields: 3 private (`gnark:",secret"`), 4 public (`gnark:",public"`)
- Field order: BirthYear, BirthMonth, BirthDay, CurrentYear, CurrentMonth, CurrentDay, MinAge
- All fields are `frontend.Variable`
- Comment at top of struct warns against reordering fields

**Define() constraints:**
- Range check on `BirthMonth`: `(BirthMonth - 1)` fits in 4 bits (allows 1–16)
- Range check on `BirthDay`: `(BirthDay - 1)` fits in 5 bits (allows 1–32)
- Range check on `BirthYear`: `(BirthYear - 1900)` fits in 8 bits (allows 1900–2155)
- Base age = `CurrentYear - BirthYear`
- Birthday comparison uses `cmp.NewBoundedComparator(api, big.NewInt(1130), false)` with MMDD encoding
- `birthdayNotPassed` derived from `comparator.IsLess(currentMMDD, birthMMDD)`
- Adjusted age = `baseAge - birthdayNotPassed`
- Final assertion: `(adjustedAge - MinAge)` fits in 8 bits

**Tests (all using `ecc.BN254`):**
- 4 happy paths: adult (birthday not passed), adult (birthday passed), exactly 18 (passed), exactly 18 (today)
- 2 minor paths: 17 (not passed), 16
- 3 range checks: invalid month (20), invalid day (50), invalid year (1800)
- 1 compilation test

### Constraints & gotchas

- **Use `std/math/cmp`, NOT `api.Cmp()`.** The `api.Cmp` method returns -1 as `p-1` in the finite field, which is error-prone to handle. The `std/math/cmp` package provides clean boolean-returning functions. This is the official recommendation from gnark docs.
- **`cmp.NewBoundedComparator` requires `math/big` import.** Don't forget it.
- **Range checks are approximate.** 4-bit month check allows 1–16, not 1–12. Document in limitations.md.
- **`api.ToBinary(x, n)` is both decomposition and assertion.** If `x ≥ 2^n`, constraint system is unsatisfiable → prover fails.
- **Field order is witness serialization order.** Do not reorder fields after Z2 runs. Add the warning comment.
- **Test with `test.WithCurves(ecc.BN254)`** to match production curve.
- **First `go test` will be slow** (~2-3 min) because gnark compiles crypto code. This is normal.
- **`cmp.IsLess` uses solver hints internally.** If you later need to verify proofs in a separate binary that doesn't import the circuit, you may need to register hints via `std.RegisterHints()`. For this project it's not an issue since the chain only calls `groth16.Verify()` which doesn't need hints.

### Definition of done

- [ ] `go test ./zk/circuit/ -v` passes all 10 test cases
- [ ] Circuit compiles: `frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &AgeCircuit{})` succeeds
- [ ] Code has comments explaining every step of `Define()`
- [ ] A non-ZK person can read `age_circuit.go` and explain the logic in 3 minutes

---

## Z2 — Key Generation (Trusted Setup) + VK Export

**Assignee:** Person B (~1-2h)
**Depends on:** Z1 (circuit must compile and pass tests)

### Context

Groth16 requires a one-time trusted setup producing:
- **Proving key (pk):** ~several MB, used by prover. Stays on prover's machine.
- **Verification key (vk):** ~few KB, used by verifier. Embedded in chain binary.

In production, setup requires multi-party computation (MPC). Here we run it
locally — a documented limitation.

This task exports the VK as a Go source file (`vk_embedded.go`) for Person A.

### Scope

**In scope:**
- Compile circuit to R1CS
- Run `groth16.Setup()` → pk + vk
- Save pk, vk, R1CS to `zk/keys/`
- Export VK as Go source file with `package keeper`
- Print constraint count to stderr

**Out of scope:**
- MPC ceremony, PLONK setup, CLI flags

### File to create

```
zk/setup/main.go
```

### Implementation — `setup/main.go`

```go
package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"

	"zk-age-verify/zk/circuit"
)

// Run from repo root: go run zk/setup/main.go
func main() {
	// ---- Step 1: Compile circuit to R1CS ----
	fmt.Fprintln(os.Stderr, "Compiling circuit...")
	var c circuit.AgeCircuit
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Circuit compilation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Circuit compiled: %d constraints\n", ccs.GetNbConstraints())

	// ---- Step 2: Groth16 trusted setup ----
	// SECURITY: In production, this MUST be a multi-party ceremony.
	// Anyone who knows the "toxic waste" from setup can forge proofs.
	fmt.Fprintln(os.Stderr, "Running Groth16 setup (this may take a moment)...")
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Setup failed: %v\n", err)
		os.Exit(1)
	}

	// ---- Step 3: Create output directory ----
	if err := os.MkdirAll("zk/keys", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to create keys directory: %v\n", err)
		os.Exit(1)
	}

	// ---- Step 4: Save R1CS ----
	writeFile("zk/keys/circuit.r1cs", func(f *os.File) error { _, err := ccs.WriteTo(f); return err })

	// ---- Step 5: Save proving key ----
	writeFile("zk/keys/proving.key", func(f *os.File) error { _, err := pk.WriteTo(f); return err })

	// ---- Step 6: Save verification key ----
	writeFile("zk/keys/verification.key", func(f *os.File) error { _, err := vk.WriteTo(f); return err })

	// ---- Step 7: Export VK as Go source file ----
	var vkBuf bytes.Buffer
	if _, err := vk.WriteTo(&vkBuf); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to serialize VK: %v\n", err)
		os.Exit(1)
	}
	goSrc := generateGoSource(vkBuf.Bytes())
	if err := os.WriteFile("zk/keys/vk_embedded.go", []byte(goSrc), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to write vk_embedded.go: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "All keys saved to zk/keys/")
	fmt.Fprintln(os.Stderr, "Next: copy zk/keys/vk_embedded.go → chain/x/ageverify/keeper/")
}

// writeFile creates a file and writes data to it using the provided function.
func writeFile(path string, writeFn func(*os.File) error) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot create %s: %v\n", path, err)
		os.Exit(1)
	}
	defer f.Close()
	if err := writeFn(f); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot write %s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Saved: %s\n", path)
}

// generateGoSource creates a Go file with the VK as a byte literal.
// Package is "keeper" so Person A drops it into chain/x/ageverify/keeper/.
func generateGoSource(vkBytes []byte) string {
	var buf bytes.Buffer
	buf.WriteString("package keeper\n\n")
	buf.WriteString("// VKBytes contains the serialized Groth16 verification key (BN254).\n")
	buf.WriteString("// Generated by: go run zk/setup/main.go\n")
	buf.WriteString("// DO NOT EDIT. Regenerate if the circuit (zk/circuit/age_circuit.go) changes.\n")
	buf.WriteString("var VKBytes = []byte{")
	for i, b := range vkBytes {
		if i%16 == 0 {
			buf.WriteString("\n\t")
		}
		fmt.Fprintf(&buf, "0x%02x, ", b)
	}
	buf.WriteString("\n}\n")
	return buf.String()
}
```

### Acceptance criteria

- [ ] Running `go run zk/setup/main.go` from repo root creates `circuit.r1cs`, `proving.key`, `verification.key` in `zk/keys/`
- [ ] `zk/keys/vk_embedded.go` is valid Go: declares `package keeper`, exports `var VKBytes = []byte{...}`
- [ ] Constraint count printed to stderr (expected: ~1000-5000 for this circuit)
- [ ] All generated files > 0 bytes

### Constraints & gotchas

- **Run from repo root.** All paths are relative to repo root.
- **If circuit changes → re-run setup → give Person A new `vk_embedded.go`.** Old keys invalid.
- **Proving key is large (several MB).** Don't commit to git.
- **First run is slow** (~30-60s) because of pairing computation.

### Definition of done

- [ ] All files generated without errors
- [ ] `vk_embedded.go` is valid Go with `package keeper`
- [ ] Person A has received `vk_embedded.go`

---

## Z3 — CLI Prover Tool

**Assignee:** Person B (~2-3h)
**Depends on:** Z1 (circuit), Z2 (keys)

### Context

User-facing tool. Runs locally, takes birth date, outputs JSON with ZK proof.
Birth date NEVER leaves the machine. No network calls. Stdout = JSON, stderr = status.

### Scope

**In scope:**
- CLI with `--year`, `--month`, `--day` flags
- Load R1CS and proving key from `zk/keys/`
- Build witness, generate Groth16 proof
- Serialize proof + public witness to base64 JSON on stdout
- Graceful error for minors
- Status messages on stderr

**Out of scope:**
- Sending transaction to chain, overriding current date, interactive input, network calls

### File to create

```
zk/prover/main.go
```

### Implementation — `prover/main.go`

```go
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"

	"zk-age-verify/zk/circuit"
)

// ProofOutput is the JSON structure written to stdout.
// The chain module expects these exact field names.
type ProofOutput struct {
	Proof          string `json:"proof"`           // base64 gnark proof
	PublicWitness  string `json:"public_witness"`  // base64 public witness
	CurrentDate    string `json:"current_date"`    // YYYYMMDD
	CircuitVersion string `json:"circuit_version"` // for compatibility
}

// Run from repo root: go run zk/prover/main.go --year 2000 --month 6 --day 15
func main() {
	year := flag.Int("year", 0, "Birth year (e.g. 2000)")
	month := flag.Int("month", 0, "Birth month (1-12)")
	day := flag.Int("day", 0, "Birth day (1-31)")
	flag.Parse()

	if *year == 0 || *month == 0 || *day == 0 {
		fmt.Fprintln(os.Stderr, "Usage: go run zk/prover/main.go --year YYYY --month MM --day DD")
		fmt.Fprintln(os.Stderr, "Example: go run zk/prover/main.go --year 2000 --month 6 --day 15")
		os.Exit(1)
	}

	now := time.Now()

	// ---- Step 1: Build assignment (all circuit inputs) ----
	fmt.Fprintln(os.Stderr, "Building witness...")
	assign := &circuit.AgeCircuit{
		BirthYear:    *year,
		BirthMonth:   *month,
		BirthDay:     *day,
		CurrentYear:  now.Year(),
		CurrentMonth: int(now.Month()),
		CurrentDay:   now.Day(),
		MinAge:       18,
	}

	// ---- Step 2: Compile circuit ----
	// Must match the circuit used during key generation (Z2).
	//
	// DO NOT replace this Compile() call with loading the R1CS from file.
	// The circuit uses std/math/cmp which registers solver hints during
	// Compile(). Loading a serialized R1CS skips hint registration and
	// Prove() will fail with a cryptic "missing hint" error.
	fmt.Fprintln(os.Stderr, "Compiling circuit...")
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit.AgeCircuit{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Circuit compilation failed: %v\n", err)
		os.Exit(1)
	}

	// ---- Step 3: Create full witness ----
	fullWitness, err := frontend.NewWitness(assign, ecc.BN254.ScalarField())
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to create witness: %v\n", err)
		os.Exit(1)
	}

	// ---- Step 4: Load proving key ----
	fmt.Fprintln(os.Stderr, "Loading proving key...")
	pkFile, err := os.Open("zk/keys/proving.key")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot open proving key: %v\n", err)
		fmt.Fprintln(os.Stderr, "Have you run 'go run zk/setup/main.go' first?")
		os.Exit(1)
	}
	defer pkFile.Close()
	pk := groth16.NewProvingKey(ecc.BN254)
	if _, err := pk.ReadFrom(pkFile); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to read proving key: %v\n", err)
		os.Exit(1)
	}

	// ---- Step 5: Generate proof ----
	// If user is under 18, constraints are unsatisfiable → Prove() errors.
	fmt.Fprintln(os.Stderr, "Generating ZK proof (this takes a few seconds)...")
	proof, err := groth16.Prove(ccs, pk, fullWitness)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Proof generation failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "This usually means the age requirement is not met (age < 18).")
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "Proof generated successfully.")

	// ---- Step 6: Extract public witness (strips private inputs) ----
	publicWitness, err := fullWitness.Public()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to extract public witness: %v\n", err)
		os.Exit(1)
	}

	// ---- Step 7: Serialize to base64 ----
	var proofBuf bytes.Buffer
	if _, err := proof.WriteTo(&proofBuf); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to serialize proof: %v\n", err)
		os.Exit(1)
	}
	var witBuf bytes.Buffer
	if _, err := publicWitness.WriteTo(&witBuf); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to serialize witness: %v\n", err)
		os.Exit(1)
	}

	// ---- Step 8: JSON to stdout ----
	output := ProofOutput{
		Proof:          base64.StdEncoding.EncodeToString(proofBuf.Bytes()),
		PublicWitness:  base64.StdEncoding.EncodeToString(witBuf.Bytes()),
		CurrentDate:    now.Format("20060102"), // Go reference time quirk
		CircuitVersion: "age-check-v1",
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: JSON encoding failed: %v\n", err)
		os.Exit(1)
	}
}
```

### Output format

```json
{
  "proof": "base64...",
  "public_witness": "base64...",
  "current_date": "20260513",
  "circuit_version": "age-check-v1"
}
```

### Acceptance criteria

- [ ] `--year 2000 --month 6 --day 15` → valid JSON on stdout, exit 0
- [ ] `--year 2015 --month 1 --day 1` → error on stderr mentioning "age requirement", exit 1
- [ ] No flags → usage on stderr, exit 1
- [ ] JSON fields: `proof`, `public_witness`, `current_date`, `circuit_version` — all non-empty strings
- [ ] Base64 uses `StdEncoding` (with padding `=`), NOT `URLEncoding` or `RawStdEncoding`
- [ ] Zero network calls (verifiable with `strace`)

### Constraints & gotchas

- **Run from repo root.** `zk/keys/proving.key` path is relative.
- **`time.Now()` is the only date source.** No `--date` flag — chain validates against block time.
- **Proof gen for minor = NO proof**, not a "false" proof. `Prove()` returns error.
- **`now.Format("20060102")` — Go time format uses reference date Jan 2 2006.** Not a bug.
- **First run compiles gnark internally (~2-3min).** Subsequent runs are fast.
- **DO NOT "optimize" by loading R1CS from file instead of Compile().** The circuit uses `std/math/cmp` which registers solver hints during `frontend.Compile()`. Loading a pre-serialized R1CS from disk skips hint registration, and `groth16.Prove()` will fail with `"missing hint"`. The in-code comment says the same thing.

### Definition of done

- [ ] Adult → JSON on stdout, exit 0
- [ ] Minor → error on stderr, exit 1
- [ ] No flags → usage on stderr, exit 1
- [ ] Output JSON is parseable, fields match contract

---

## Z4 — Standalone Proof Verifier (Pre-Integration Sanity Check)

**Assignee:** Person B (~30min)
**Depends on:** Z2 (VK), Z3 (prover generates proof.json)

### Context

This is a small diagnostic tool, not a deliverable. It simulates exactly what
the chain will do: load VK, base64-decode the proof and witness from Z3's
JSON output, call `groth16.Verify`. If this passes, Person B knows that the
serialized proof is valid before integration. If O5 then fails, the bug is
on Person A's chain side, not in the proof.

Without this, integration debugging is a guessing game.

### File to create

```
zk/verify/main.go
```

### Implementation

```go
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/backend/witness"
)

type ProofInput struct {
	Proof         string `json:"proof"`
	PublicWitness string `json:"public_witness"`
}

// Run from repo root: go run zk/prover/main.go --year 2000 --month 6 --day 15 | go run zk/verify/main.go
// Or: go run zk/verify/main.go < proof.json
func main() {
	// Read JSON from stdin
	var input ProofInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot parse JSON from stdin: %v\n", err)
		os.Exit(1)
	}

	// Decode base64
	proofBytes, err := base64.StdEncoding.DecodeString(input.Proof)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot decode proof base64: %v\n", err)
		os.Exit(1)
	}
	witnessBytes, err := base64.StdEncoding.DecodeString(input.PublicWitness)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot decode witness base64: %v\n", err)
		os.Exit(1)
	}

	// Load verification key (same file the chain will use)
	vkFile, err := os.Open("zk/keys/verification.key")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot open VK: %v\n", err)
		os.Exit(1)
	}
	defer vkFile.Close()
	vk := groth16.NewVerifyingKey(ecc.BN254)
	if _, err := vk.ReadFrom(vkFile); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot read VK: %v\n", err)
		os.Exit(1)
	}

	// Deserialize proof
	proof := groth16.NewProof(ecc.BN254)
	if _, err := proof.ReadFrom(bytes.NewReader(proofBytes)); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot deserialize proof: %v\n", err)
		os.Exit(1)
	}

	// Deserialize public witness
	pubWitness, err := witness.New(ecc.BN254.ScalarField())
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot create witness: %v\n", err)
		os.Exit(1)
	}
	if _, err := pubWitness.ReadFrom(bytes.NewReader(witnessBytes)); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot deserialize witness: %v\n", err)
		os.Exit(1)
	}

	// VERIFY — this is the same call the chain makes
	if err := groth16.Verify(proof, vk, pubWitness); err != nil {
		fmt.Fprintf(os.Stderr, "VERIFICATION FAILED: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "VERIFICATION PASSED — proof is valid.")
	fmt.Fprintln(os.Stderr, "The chain should accept this proof during integration (O5).")
}
```

### Usage

```bash
# Generate proof and verify in one pipeline:
go run zk/prover/main.go --year 2000 --month 6 --day 15 | go run zk/verify/main.go

# Or two steps:
go run zk/prover/main.go --year 2000 --month 6 --day 15 > /tmp/proof.json
go run zk/verify/main.go < /tmp/proof.json
```

### Acceptance criteria

- [ ] Valid proof JSON piped in → prints "VERIFICATION PASSED", exit 0
- [ ] Corrupted JSON piped in → prints error, exit 1
- [ ] This tool uses the SAME deserialization path as C1's `verifier.go` (base64 → bytes → gnark deserialize → groth16.Verify)

### Definition of done

- [ ] Person B has verified at least one proof before O5 starts

---

## C1 — Chain Module: Keeper Logic, State, Query

**Assignee:** Person A (~6-8h)
**Depends on:** O2 (scaffold + SCAFFOLD_NOTES.md filled in), Z2 (VK file from Person B)

### IMPORTANT: Before starting C1

When opening a new chat for this task, paste THREE things:
1. The **Project Context** section (top of this document)
2. This **C1 spec** (everything below until the next `---`)
3. The contents of **`chain/SCAFFOLD_NOTES.md`** from O2

The AI will adapt the code below to match your exact scaffold output.
Without SCAFFOLD_NOTES.md, the code may have wrong import paths, wrong
Keeper field names, or wrong function signatures.

### Context

After Ignite scaffolds the module (O2), handler files are stubs. This task
fills in the real logic across five files:

1. **errors.go** — typed error codes
2. **keys.go** — KV store prefix constant
3. **verifier.go** — loads embedded VK, wraps `groth16.Verify()`
4. **verification_store.go** — set/get verification records in KV store
5. **msg_server_submit_age_proof.go** — main handler: decode, validate, verify, store
6. **query_verification_status.go** — read verification status

**IMPORTANT: Cosmos SDK v0.50 / Ignite v28 changed KV store access.**
The keeper uses `storeService` (not the old `storeKey`). Access pattern:
```go
storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
store := prefix.NewStore(storeAdapter, []byte("prefix"))
```
Check the scaffolded `keeper.go` to see exactly what fields the Keeper struct has.

### Scope

**In scope:**
- All six files listed above
- gnark dependency added to chain's `go.mod`
- `vk_embedded.go` copied from Person B

**Out of scope:**
- Scaffolding (O2), circuit/prover (Z1-Z3)
- Replay protection, nullifiers, expiration
- Frontend, REST API

### Files to create or modify

```
chain/x/ageverify/keeper/vk_embedded.go                 # FROM Person B
chain/x/ageverify/keeper/verifier.go                    # New
chain/x/ageverify/keeper/verification_store.go          # New
chain/x/ageverify/keeper/msg_server_submit_age_proof.go # Modify (fill stub)
chain/x/ageverify/keeper/query_verification_status.go   # Modify (fill stub)
chain/x/ageverify/types/errors.go                       # New or modify
chain/x/ageverify/types/keys.go                         # Modify (add prefix)
chain/go.mod                                            # Add gnark dep
```

### Implementation — `types/errors.go`

```go
package types

import "cosmossdk.io/errors"

var (
	ErrInvalidProof            = errors.Register(ModuleName, 1100, "invalid proof encoding")
	ErrInvalidWitness          = errors.Register(ModuleName, 1101, "invalid witness encoding")
	ErrDateMismatch            = errors.Register(ModuleName, 1102, "proof date does not match block time")
	ErrProofVerificationFailed = errors.Register(ModuleName, 1103, "ZK proof verification failed")
)
```

### Implementation — `types/keys.go` (add to existing)

```go
// Add this to the existing keys.go:

// VerifiedKeyPrefix is the KV store prefix for verification records.
// Full key: VerifiedKeyPrefix + bech32address → RFC3339 timestamp string
const VerifiedKeyPrefix = "verified/"
```

### Implementation — `keeper/verifier.go`

```go
package keeper

import (
	"bytes"
	"fmt"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	gnarkwitness "github.com/consensys/gnark/backend/witness"
)

// LoadVerificationKey deserializes VKBytes (from vk_embedded.go) into a
// gnark VerifyingKey. VKBytes is generated by Person B (go run zk/setup/main.go).
func LoadVerificationKey() (groth16.VerifyingKey, error) {
	vk := groth16.NewVerifyingKey(ecc.BN254)
	if _, err := vk.ReadFrom(bytes.NewReader(VKBytes)); err != nil {
		return nil, fmt.Errorf("failed to load verification key: %w", err)
	}
	return vk, nil
}

// VerifyAgeProof checks a ZK proof against the embedded verification key.
// Returns nil if valid, error otherwise.
func VerifyAgeProof(vk groth16.VerifyingKey, proofBytes, witnessBytes []byte) error {
	// Deserialize proof
	proof := groth16.NewProof(ecc.BN254)
	if _, err := proof.ReadFrom(bytes.NewReader(proofBytes)); err != nil {
		return fmt.Errorf("cannot deserialize proof: %w", err)
	}

	// Deserialize public witness
	pubWitness, err := gnarkwitness.New(ecc.BN254.ScalarField())
	if err != nil {
		return fmt.Errorf("cannot create witness: %w", err)
	}
	if _, err := pubWitness.ReadFrom(bytes.NewReader(witnessBytes)); err != nil {
		return fmt.Errorf("cannot deserialize witness: %w", err)
	}

	// Verify proof
	if err := groth16.Verify(proof, vk, pubWitness); err != nil {
		return fmt.Errorf("proof verification failed: %w", err)
	}
	return nil
}
```

### Implementation — `keeper/verification_store.go`

**NOTE:** This code uses the Cosmos SDK v0.50 / Ignite v28 KV store pattern.
The Keeper struct generated by Ignite v28 has a `storeService` field, NOT
the old `storeKey`. If your scaffolded Keeper struct has different field
names, adapt accordingly — check `chain/x/ageverify/keeper/keeper.go`.

```go
package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"

	"ageverify/x/ageverify/types"
	// ^^^ ADAPT THIS IMPORT PATH to match your chain's go.mod module name.
	// Ignite v28 uses just the chain name (e.g. "ageverify/x/ageverify/types").
	// Check chain/go.mod for the exact module path.
)

// SetVerified stores a verification timestamp for the given address.
// Overwrites any previous record.
func (k Keeper) SetVerified(ctx context.Context, address string, timestamp string) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.VerifiedKeyPrefix))
	store.Set([]byte(address), []byte(timestamp))
}

// GetVerified returns the verification status for an address.
// Returns (true, timestamp, true) if verified, (false, "", false) if not.
func (k Keeper) GetVerified(ctx context.Context, address string) (bool, string, bool) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.VerifiedKeyPrefix))
	bz := store.Get([]byte(address))
	if bz == nil {
		return false, "", false
	}
	return true, string(bz), true
}
```

### Implementation — `keeper/msg_server_submit_age_proof.go`

**NOTE:** Ignite generates a stub for this file. Keep the function signature
that Ignite generated; only replace the body.

```go
package keeper

import (
	"context"
	"encoding/base64"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"ageverify/x/ageverify/types"
	// ^^^ ADAPT import path to match chain/go.mod
)

const dateTimeTolerance = 24 * time.Hour

func (k msgServer) SubmitAgeProof(
	goCtx context.Context,
	msg *types.MsgSubmitAgeProof,
) (*types.MsgSubmitAgeProofResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// ---- Step 1: Decode proof from base64 ----
	proofBytes, err := base64.StdEncoding.DecodeString(msg.Proof)
	if err != nil {
		return nil, types.ErrInvalidProof.Wrap("base64 decode failed")
	}

	// ---- Step 2: Decode public witness from base64 ----
	witnessBytes, err := base64.StdEncoding.DecodeString(msg.PublicWitness)
	if err != nil {
		return nil, types.ErrInvalidWitness.Wrap("base64 decode failed")
	}

	// ---- Step 3: Validate current_date vs block time ----
	// Prevents prover from using a future date to fake age.
	proofDate, err := time.Parse("20060102", msg.CurrentDate)
	if err != nil {
		return nil, types.ErrDateMismatch.Wrap("invalid format, expected YYYYMMDD")
	}
	blockTime := ctx.BlockTime()
	diff := blockTime.Sub(proofDate)
	if diff < 0 {
		diff = -diff
	}
	if diff > dateTimeTolerance {
		return nil, types.ErrDateMismatch.Wrap("proof date too far from block time")
	}

	// ---- Step 4: Load verification key ----
	vk, err := LoadVerificationKey()
	if err != nil {
		return nil, types.ErrProofVerificationFailed.Wrap(err.Error())
	}

	// ---- Step 5: Verify ZK proof ----
	// groth16.Verify checks that the proof was generated by a valid circuit
	// (matching VK) and that the private inputs satisfy the constraints
	// (age >= 18) — all without seeing the birth date.
	if err := VerifyAgeProof(vk, proofBytes, witnessBytes); err != nil {
		return nil, types.ErrProofVerificationFailed.Wrap(err.Error())
	}

	// ---- Step 6: Store result ----
	k.Keeper.SetVerified(goCtx, msg.Creator, blockTime.Format(time.RFC3339))

	// ---- Step 7: Emit event ----
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"age_verified",
		sdk.NewAttribute("address", msg.Creator),
		sdk.NewAttribute("verified_at", blockTime.Format(time.RFC3339)),
	))

	return &types.MsgSubmitAgeProofResponse{}, nil
}
```

### Implementation — `keeper/query_verification_status.go`

```go
package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ageverify/x/ageverify/types"
	// ^^^ ADAPT import path
)

func (k Keeper) VerificationStatus(
	goCtx context.Context,
	req *types.QueryVerificationStatusRequest,
) (*types.QueryVerificationStatusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	verified, verifiedAt, found := k.GetVerified(goCtx, req.Address)
	if !found {
		return &types.QueryVerificationStatusResponse{
			Verified:   false,
			VerifiedAt: "",
		}, nil
	}
	return &types.QueryVerificationStatusResponse{
		Verified:   verified,
		VerifiedAt: verifiedAt,
	}, nil
}
```

### Acceptance criteria

**Errors:**
- `ErrInvalidProof` (1100), `ErrInvalidWitness` (1101), `ErrDateMismatch` (1102), `ErrProofVerificationFailed` (1103)

**State storage:**
- `SetVerified` + `GetVerified` roundtrip works
- Non-existent address → `(false, "", false)`
- Second `SetVerified` for same address overwrites

**Message handler:**
- Valid proof + date → tx succeeds, address verified
- Invalid base64 proof → error 1100
- Invalid base64 witness → error 1101
- Date >24h from block time → error 1102
- Corrupted proof bytes → error 1103
- Event `age_verified` emitted

**Query handler:**
- Verified address → `{verified: true, verified_at: "<timestamp>"}`
- Unknown address → `{verified: false, verified_at: ""}`

### Constraints & gotchas

- **THIS CODE HAS THREE SCAFFOLD-DEPENDENT HOLES — SCAFFOLD_NOTES.md RESOLVES THEM ALL:**
  1. `storeService.OpenKVStore(ctx)` — may return `(KVStore, error)` or just `KVStore` depending on SDK minor version. SCAFFOLD_NOTES.md tells you which.
  2. `k.Keeper.SetVerified(goCtx, ...)` — uses `context.Context` but your SDK version may need `sdk.Context`. SCAFFOLD_NOTES.md tells you.
  3. `*types.MsgSubmitAgeProofResponse` — Ignite generates this name from protobuf. If Ignite chose a different name, the code won't compile. SCAFFOLD_NOTES.md has the exact name.
  When you paste SCAFFOLD_NOTES.md into the coding chat, the AI will fix all three automatically. Without it, you'll be debugging blind.
- **ADAPT IMPORT PATHS.** Ignite v28 generates the Go module as just the chain name (e.g. `ageverify`). Your import paths will be `ageverify/x/ageverify/types`, NOT `zk-age-verify/chain/x/ageverify/types`. Check SCAFFOLD_NOTES.md item #1.
- **Keeper struct fields.** Ignite v28 generates `storeService` on the Keeper. If your scaffold has a different name (unlikely but possible), check SCAFFOLD_NOTES.md item #2. The `SetVerified`/`GetVerified` code uses `k.storeService.OpenKVStore(ctx)`.
- **`runtime.KVStoreAdapter` import.** In Cosmos SDK v0.50.x: `"github.com/cosmos/cosmos-sdk/runtime"`. If missing, run `go get github.com/cosmos/cosmos-sdk/runtime`.
- **`prefix.NewStore` import.** In SDK v0.50.x: `"cosmossdk.io/store/prefix"`. NOT the old `github.com/cosmos/cosmos-sdk/store/prefix`.
- **Adding gnark to chain's go.mod.** Run `cd chain && go get github.com/consensys/gnark@v0.14.0`. If dependency conflicts arise with Cosmos SDK, add `replace` directives in `chain/go.mod`. This is ugly but standard for projects mixing gnark with Cosmos SDK.
- **`SetVerified` takes `context.Context`, not `sdk.Context`.** In SDK v0.50, keeper methods receive `context.Context`. The `storeService.OpenKVStore` accepts `context.Context` directly. The message handler unwraps to `sdk.Context` only for event emission and block time.
- **Loading VK on every tx is simple but fine for this project.** Production: load once in keeper constructor.

### Definition of done

- [ ] `ignite chain build` succeeds with all files
- [ ] Valid proof → tx accepted → address verified in state → queryable
- [ ] Invalid inputs → specific error codes
- [ ] Code commented at every step

---

# OPERATIONAL TASKS

---

## O0 — Dockerfile (Pinned Environment)

**Assignee:** Both (~1h)

### Why

Eliminates "works on my machine" problems. Pins Go, Ignite, and gnark
versions identically for both people. Also prevents the "first gnark
build takes 5 minutes and looks frozen" surprise.

### Dockerfile

```dockerfile
FROM golang:1.22-bookworm

# Install tools
RUN apt-get update && apt-get install -y jq curl git && rm -rf /var/lib/apt/lists/*

# Install Ignite CLI v28 (latest patch)
RUN curl https://get.ignite.com/cli@v28 | bash

# Pre-download gnark to cache compilation
RUN mkdir /tmp/gnark-warmup && cd /tmp/gnark-warmup && \
    go mod init warmup && \
    go get github.com/consensys/gnark@v0.14.0 && \
    go get github.com/consensys/gnark-crypto@v0.20.0 && \
    go build github.com/consensys/gnark/... && \
    rm -rf /tmp/gnark-warmup

WORKDIR /workspace
```

### docker-compose.yml

```yaml
version: "3.8"
services:
  dev:
    build: .
    volumes:
      - .:/workspace
    ports:
      - "1317:1317"    # Cosmos REST API
      - "26657:26657"  # CometBFT RPC
    command: sleep infinity
```

### Usage

```bash
# Build once
docker compose build

# Enter container
docker compose run --rm dev bash

# Inside container — everything works:
ignite version          # v28.x.x
go version              # go1.22.x
go run zk/setup/main.go # gnark cached, fast
ignite chain serve      # chain starts
```

### Acceptance criteria

- [ ] `docker compose build` completes without errors
- [ ] Inside container: `go version` = 1.22.x, `ignite version` = v28.x
- [ ] Inside container: `go get github.com/consensys/gnark@v0.14.0` is instant (cached)
- [ ] Both people use the same image

---

## O1 — Repository Setup

**Assignee:** Both (~2h, pair session)

```bash
# 1. Create repo
mkdir zk-age-verify && cd zk-age-verify && git init

# 2. Add Dockerfile + docker-compose.yml (from O0)

# 3. Enter container
docker compose run --rm dev bash

# 4. Scaffold chain
ignite scaffold chain ageverify --path ./chain

# 5. Init ZK module
mkdir -p zk/circuit zk/setup zk/prover zk/keys
cd zk && go mod init zk-age-verify/zk
go get github.com/consensys/gnark@v0.14.0
go get github.com/consensys/gnark-crypto@v0.20.0
cd ..

# 6. Go workspace
go work init && go work use ./chain && go work use ./zk

# 7. Verify
cd chain && ignite chain serve    # blocks produced? ctrl+c
cd ../zk && go build ./...        # compiles?

# 8. .gitignore
echo "zk/keys/*.key\nzk/keys/*.r1cs\nzk/keys/vk_embedded.go" >> .gitignore

# 9. Branches
git add -A && git commit -m "initial scaffold"
git checkout -b feature/chain-module   # Person A
git checkout main
git checkout -b feature/zk-circuit     # Person B
```

**Done when:** Both people can `docker compose run --rm dev bash`, build chain, build ZK module.

---

## O2 — Scaffold Custom Module

**Assignee:** Person A (~1-2h)

```bash
cd chain/
ignite scaffold module ageverify
ignite scaffold message submit-age-proof \
    proof public_witness current_date \
    --module ageverify
ignite scaffold query verification-status address \
    --module ageverify \
    --response verified:bool,verified_at:string
ignite chain build
```

**After scaffolding, IMMEDIATELY check these six things and write them down.**
These determine the exact code in C1. If they differ from C1's assumptions,
paste them into the C1 coding chat so the AI adapts.

1. **Module name:** `chain/go.mod` line 1 → e.g. `module ageverify`. This is
   the root of all import paths. C1 assumes `ageverify/x/ageverify/types`.
   If your go.mod says something else, all C1 imports change.

2. **Keeper struct fields:** Open `chain/x/ageverify/keeper/keeper.go`. Find
   the `Keeper` struct. Does it have `storeService` or `storeKey`? What is
   the type? (Expect `store.KVStoreService` in SDK v0.50.) C1's
   `verification_store.go` calls `k.storeService.OpenKVStore(ctx)`.

3. **OpenKVStore return type:** In the same file, find any existing usage of
   `storeService.OpenKVStore(ctx)`. Does it return `(KVStore, error)` or just
   `KVStore`? In SDK v0.50.x some versions return a tuple, others return
   just the store. C1 code must match. If it returns an error, wrap it:
   `store, err := k.storeService.OpenKVStore(ctx)`.

4. **Message handler stub signature:** Open
   `chain/x/ageverify/keeper/msg_server_submit_age_proof.go`. Copy the EXACT
   function signature — return type name, parameter types. C1 must match
   this exactly. Expected: `func (k msgServer) SubmitAgeProof(ctx context.Context, msg *types.MsgSubmitAgeProof) (*types.MsgSubmitAgeProofResponse, error)`.

5. **Query handler stub signature:** Open
   `chain/x/ageverify/keeper/query_verification_status.go`. Same — copy the
   exact signature. Expected: `func (k Keeper) VerificationStatus(ctx context.Context, req *types.QueryVerificationStatusRequest) (*types.QueryVerificationStatusResponse, error)`.

6. **Generated types:** Check `chain/x/ageverify/types/`. What message and
   response types did Ignite create? Look for `MsgSubmitAgeProof`,
   `MsgSubmitAgeProofResponse`, `QueryVerificationStatusRequest`,
   `QueryVerificationStatusResponse`. Verify field names: `Proof`,
   `PublicWitness`, `CurrentDate` (Ignite converts snake_case proto to
   CamelCase Go). Also check `keys.go` for existing constants.

**Save these results in a file `chain/SCAFFOLD_NOTES.md`.** When starting
the C1 task, paste the Project Context + C1 spec + the contents of
SCAFFOLD_NOTES.md into the chat. This lets the AI adapt C1 code to your
exact scaffold output.

**Done when:** Chain compiles and starts. `ageverifyd tx ageverify submit-age-proof` and `ageverifyd query ageverify verification-status` commands exist. `SCAFFOLD_NOTES.md` is filled in.

---

## O5 — Integration

**Assignee:** Both (~4-5h, pair session — CRITICAL PATH)

**Pre-requisites:** C1 done, Z1+Z2+Z3+Z4 done (Person B has verified a proof locally).

```bash
# 1. Copy VK
cp zk/keys/vk_embedded.go chain/x/ageverify/keeper/

# 2. Build chain with gnark
cd chain && go get github.com/consensys/gnark@v0.14.0 && go mod tidy && ignite chain build
# ^^^ IF THIS FAILS: check error, likely dependency conflict.
# Fix with replace directives in chain/go.mod.

# 3. Start chain
ignite chain serve &

# 4. Generate proof
go run zk/prover/main.go --year 2000 --month 6 --day 15 > /tmp/proof.json

# 5. Submit to chain
PROOF=$(jq -r .proof /tmp/proof.json)
WITNESS=$(jq -r .public_witness /tmp/proof.json)
DATE=$(jq -r .current_date /tmp/proof.json)
ageverifyd tx ageverify submit-age-proof "$PROOF" "$WITNESS" "$DATE" \
    --from alice -y --chain-id ageverify

# 6. Query
ADDR=$(ageverifyd keys show alice -a)
ageverifyd query ageverify verification-status "$ADDR"
# Expected: verified: true

# 7. Test rejection (random bytes)
FAKE=$(echo "not-a-proof" | base64)
ageverifyd tx ageverify submit-age-proof "$FAKE" "$WITNESS" "$DATE" \
    --from bob -y --chain-id ageverify
# Expected: transaction fails
```

**Common integration bugs:**

| Symptom | Cause | Fix |
|---|---|---|
| `go mod tidy` fails | gnark vs Cosmos SDK dep conflict | Add `replace` directives in chain/go.mod for conflicting packages |
| `proof verification failed` on valid proof | Witness field order mismatch | Ensure AgeCircuit field order unchanged since Z2 |
| `invalid witness size` | Circuit recompiled after keygen | Re-run Z2, re-copy vk_embedded.go |
| Build fails: `storeKey undefined` | Code uses old SDK pattern | Use `storeService.OpenKVStore(ctx)` per SDK v0.50 |
| Tx fails: `unknown message type` | Module not registered | Check `app/app.go` — Ignite should auto-register |

**Done when:** Valid proof → verified. Invalid proof → rejected. Both people can reproduce.

---

## O6 — Demo Script

**Assignee:** Both (~1-2h)

Create `scripts/demo.sh`. Assumes chain is running. Runs happy path + rejection. Clear output at each step.

**Done when:** `bash scripts/demo.sh` runs unattended, output readable by examiner.

---

## O7 — Architecture Documentation

**Assignee:** Person A (~2h)

`README.md`: description, prerequisites, setup, how to run demo.
`docs/architecture.md`: component diagram, data flow, tech choices.

---

## O8 — Limitations & Security

**Assignee:** Person B (~2h)

`docs/limitations.md` covering:

1. **No trusted issuer** — user can lie. Fix: eID + EdDSA in circuit.
2. **Single-party setup** — can forge proofs. Fix: MPC ceremony or PLONK.
3. **Replay attacks** — proof reusable from other address. Fix: bind address as public input.
4. **Block time reliance** — approximate. Fix: tighter tolerance, oracle.
5. **No revocation** — verified forever. Fix: expiration block height.
6. **Approximate range checks** — month allows 1-16. Fix: tighter constraints.

Each: what, why it matters, production mitigation.

---

## Dependency Graph

```
O0 (Dockerfile, both)
└── O1 (setup, both)
    ├── O2 (scaffold, Person A)
    │   └── C1 (chain module, Person A)
    │
    └── Z1 (circuit, Person B)
        └── Z2 (keygen, Person B)
            └── Z3 (CLI prover, Person B)
                └── Z4 (standalone verify, Person B)

C1 + Z4 (both sides verified independently)
└── O5 (integration, both)
    ├── O6 (demo script, both)
    ├── O7 (architecture docs, Person A)
    └── O8 (limitations docs, Person B)
```

---

## Known Pitfalls (reference sheet)

| # | Pitfall | Who | When | What to do |
|---|---------|-----|------|------------|
| 1 | First `go build` of gnark takes 3-5 min | Both | O1 | Normal. Don't ctrl+C. Docker O0 pre-warms this. |
| 2 | `go run` from wrong directory → file not found | Both | Z2, Z3 | Always run from repo root. Comment at top of each main.go. |
| 3 | Different gnark versions → proof incompatibility | Both | O5 | Pin `@v0.14.0` in BOTH go.mod files. Docker helps. |
| 4 | Circuit field reorder → silent verify failure | Person B | After Z2 | Never reorder AgeCircuit fields. Warning comment in struct. |
| 5 | Ignite storeService vs storeKey | Person A | C1 | Check scaffolded keeper.go. v28/SDK v0.50 uses storeService. |
| 6 | Import path mismatch | Person A | C1 | Check chain/go.mod line 1. Use that as import root. |
| 7 | gnark + Cosmos SDK dep conflict | Person A | O5 | Use `replace` directives in chain/go.mod. |
| 8 | `jq` not installed | Both | O5, O6 | Install in Dockerfile (already included in O0). |
| 9 | Base64 encoding mismatch (Std vs URL vs Raw) | Both | O5 | Both sides use `base64.StdEncoding` with padding. |
| 10 | `--chain-id` flag required in Ignite v28 | Person A | O5 | Add `--chain-id ageverify` to all `ageverifyd tx` commands. |
| 11 | Replacing `frontend.Compile()` with loading R1CS from file in prover | Person B | Z3 | DON'T. `std/math/cmp` registers solver hints during Compile(). Loading serialized R1CS skips this → cryptic "missing hint" error. |
| 12 | Starting C1 without SCAFFOLD_NOTES.md | Person A | C1 | O2 generates scaffold. Fill in SCAFFOLD_NOTES.md (6 items). Paste into C1 coding chat. Without it, import paths, Keeper fields, and return types may be wrong. |
| 13 | `OpenKVStore` return type varies across SDK v0.50.x patches | Person A | C1 | Some return `(KVStore, error)`, some return `KVStore`. Check SCAFFOLD_NOTES.md item #3. |
