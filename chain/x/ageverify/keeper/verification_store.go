package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"

	"ageverify/x/ageverify/types"
)

// SetVerified stores a verification timestamp for the given address.
func (k Keeper) SetVerified(ctx context.Context, address string, timestamp string) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.VerifiedKeyPrefix))
	store.Set([]byte(address), []byte(timestamp))
}

// GetVerified returns the verification status for an address.
// Returns (verified, timestamp, found).
func (k Keeper) GetVerified(ctx context.Context, address string) (bool, string, bool) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.VerifiedKeyPrefix))
	bz := store.Get([]byte(address))
	if bz == nil {
		return false, "", false
	}
	return true, string(bz), true
}
