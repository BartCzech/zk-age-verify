package keeper

import (
	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	logger       log.Logger
}

func NewKeeper(cdc codec.BinaryCodec, storeService store.KVStoreService, logger log.Logger) Keeper {
	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		logger:       logger,
	}
}

func (k Keeper) Logger() log.Logger {
	return k.logger.With("module", "x/ageverify")
}
