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
	gnarklogger "github.com/consensys/gnark/logger"
	"github.com/rs/zerolog"

	"zk-age-verify/zk/circuit"
)

// ProofOutput is the JSON structure written to stdout.
// The chain module expects these exact field names.
type ProofOutput struct {
	Proof          string `json:"proof"`
	PublicWitness  string `json:"public_witness"`
	CurrentDate    string `json:"current_date"`
	CircuitVersion string `json:"circuit_version"`
}

// Run from repo root: go run zk/prover/main.go --year 2000 --month 6 --day 15
func main() {
	year   := flag.Int("year", 0, "Birth year (e.g. 2000)")
	month  := flag.Int("month", 0, "Birth month (1-12)")
	day    := flag.Int("day", 0, "Birth day (1-31)")
	minAge := flag.Int("min-age", 18, "Minimum age required (default 18)")
	flag.Parse()

	gnarklogger.Set(zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}).With().Timestamp().Logger())

	if *year == 0 || *month == 0 || *day == 0 {
		fmt.Fprintln(os.Stderr, "Usage: go run zk/prover/main.go --year YYYY --month MM --day DD [--min-age N]")
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
		MinAge:       *minAge,
	}

	// ---- Step 2: Compile circuit ----
	// DO NOT replace Compile() with loading R1CS from file.
	// std/math/cmp registers solver hints during Compile(). Loading a
	// serialized R1CS skips hint registration → Prove() fails with
	// a cryptic "missing hint" error.
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
		CurrentDate:    now.Format("20060102"), // Go reference time quirk: Jan 2 2006
		CircuitVersion: "age-check-v1",
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: JSON encoding failed: %v\n", err)
		os.Exit(1)
	}
}
