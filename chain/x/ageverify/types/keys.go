package types

const (
	ModuleName = "ageverify"
	StoreKey   = ModuleName

	// VerifiedKeyPrefix is the KV prefix for verification records.
	// Full key: VerifiedKeyPrefix + bech32address → RFC3339 timestamp
	VerifiedKeyPrefix = "verified/"
)
