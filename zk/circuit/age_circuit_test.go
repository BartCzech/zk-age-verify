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
