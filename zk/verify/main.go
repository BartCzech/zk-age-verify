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

// Run from repo root:
//
//	go run zk/prover/main.go --year 2000 --month 6 --day 15 | go run zk/verify/main.go
//	go run zk/verify/main.go < proof.json
//
// NOTE: zk/keys/verification.key must match the key embedded in the chain node.
// Re-run zk/setup/main.go and restart the chain if you regenerate the keys.
func main() {
	// ---- Read JSON from stdin ----
	var input ProofInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot parse JSON from stdin: %v\n", err)
		os.Exit(1)
	}

	// ---- Decode base64 ----
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

	// ---- Load verification key (same file the chain will use) ----
	vkFile, err := os.Open("zk/keys/verification.key")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot open verification key: %v\n", err)
		fmt.Fprintln(os.Stderr, "Have you run 'go run zk/setup/main.go' first?")
		os.Exit(1)
	}
	defer vkFile.Close()
	vk := groth16.NewVerifyingKey(ecc.BN254)
	if _, err := vk.ReadFrom(vkFile); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot read VK: %v\n", err)
		os.Exit(1)
	}

	// ---- Deserialize proof ----
	proof := groth16.NewProof(ecc.BN254)
	if _, err := proof.ReadFrom(bytes.NewReader(proofBytes)); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot deserialize proof: %v\n", err)
		os.Exit(1)
	}

	// ---- Deserialize public witness ----
	pubWitness, err := witness.New(ecc.BN254.ScalarField())
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot create witness: %v\n", err)
		os.Exit(1)
	}
	if _, err := pubWitness.ReadFrom(bytes.NewReader(witnessBytes)); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot deserialize witness: %v\n", err)
		os.Exit(1)
	}

	// ---- Verify — same call the chain makes ----
	if err := groth16.Verify(proof, vk, pubWitness); err != nil {
		fmt.Fprintf(os.Stderr, "VERIFICATION FAILED: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "VERIFICATION PASSED — proof is valid.")
}
