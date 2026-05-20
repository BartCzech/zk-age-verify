package types

import "cosmossdk.io/errors"

var (
	ErrInvalidProof            = errors.Register(ModuleName, 1100, "invalid proof encoding")
	ErrInvalidWitness          = errors.Register(ModuleName, 1101, "invalid witness encoding")
	ErrDateMismatch            = errors.Register(ModuleName, 1102, "proof date does not match block time")
	ErrProofVerificationFailed = errors.Register(ModuleName, 1103, "ZK proof verification failed")
)
