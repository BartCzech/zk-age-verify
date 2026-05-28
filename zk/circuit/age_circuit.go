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
	//   Exact checks would cost extra constraints. Documented in limitations.md.
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
	// cmp.IsLess(api, a, b) returns 1 if a < b, 0 if a >= b.
	// Use BoundedComparator for efficiency — max diff is 1231-101 = 1130.
	// Second arg (false) = deterministic mode.
	// ---------------------------------------------------------------

	currentMMDD := api.Add(api.Mul(c.CurrentMonth, 100), c.CurrentDay)
	birthMMDD := api.Add(api.Mul(c.BirthMonth, 100), c.BirthDay)

	mmddComparator := cmp.NewBoundedComparator(api, big.NewInt(1130), false)

	// birthdayNotPassed = 1 if currentMMDD < birthMMDD (birthday not yet)
	// birthdayNotPassed = 0 if currentMMDD >= birthMMDD (birthday passed or today)
	birthdayNotPassed := mmddComparator.IsLess(currentMMDD, birthMMDD)

	// ---------------------------------------------------------------
	// STEP 4: Adjusted age.
	//
	// adjustedAge = baseAge - birthdayNotPassed
	// If birthday passed (0): adjustedAge = baseAge
	// If birthday not passed (1): adjustedAge = baseAge - 1
	// ---------------------------------------------------------------

	adjustedAge := api.Sub(baseAge, birthdayNotPassed)

	// ---------------------------------------------------------------
	// STEP 5: Assert adjustedAge >= minAge.
	//
	// diff = adjustedAge - minAge must fit in 8 bits (range 0-255).
	// If adjustedAge < minAge, diff is "negative" in the finite field
	// (~254 bits) — ToBinary(diff, 8) fails the constraint.
	// 8 bits allows max provable age of minAge+255 = 273. More than enough.
	// ---------------------------------------------------------------

	diff := api.Sub(adjustedAge, c.MinAge)
	api.ToBinary(diff, 8)

	return nil
}
