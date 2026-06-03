package app

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"

	"github.com/cosmos/gogoproto/proto"
	"cosmossdk.io/x/tx/signing"

	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/consensus"
	consensuskeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	consensustypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	ageverifykeeper "ageverify/x/ageverify/keeper"
	ageverifymodule "ageverify/x/ageverify/module"
	ageverifytypes "ageverify/x/ageverify/types"
)

const Name = "ageverify"

var (
	DefaultNodeHome string

	ModuleBasics = module.NewBasicManager(
		auth.AppModuleBasic{},
		genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
		bank.AppModuleBasic{},
		staking.AppModuleBasic{},
		params.AppModuleBasic{},
		consensus.AppModuleBasic{},
		ageverifymodule.AppModuleBasic{},
	)
)

func init() {
	userHomeDir, _ := os.UserHomeDir()
	DefaultNodeHome = filepath.Join(userHomeDir, ".ageverify")
}

var _ servertypes.Application = (*App)(nil)

type App struct {
	*baseapp.BaseApp

	cdc               *codec.ProtoCodec
	legacyAmino       *codec.LegacyAmino
	interfaceRegistry codectypes.InterfaceRegistry
	// client.TxConfig, not sdk.TxConfig — TxConfig lives in the client package in SDK v0.50
	txConfig client.TxConfig

	keys  map[string]*storetypes.KVStoreKey
	tkeys map[string]*storetypes.TransientStoreKey

	AccountKeeper   authkeeper.AccountKeeper
	BankKeeper      bankkeeper.BaseKeeper
	StakingKeeper   *stakingkeeper.Keeper
	ParamsKeeper    paramskeeper.Keeper
	ConsensusKeeper consensuskeeper.Keeper

	AgeVerifyKeeper ageverifykeeper.Keeper

	mm           *module.Manager
	configurator module.Configurator
}

func NewApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) *App {
	interfaceRegistry, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: signing.Options{
			AddressCodec:          authcodec.NewBech32Codec(sdk.Bech32PrefixAccAddr),
			ValidatorAddressCodec: authcodec.NewBech32Codec(sdk.Bech32PrefixValAddr),
		},
	})
	if err != nil {
		panic(err)
	}
	appCodec := codec.NewProtoCodec(interfaceRegistry)
	legacyAmino := codec.NewLegacyAmino()

	ModuleBasics.RegisterInterfaces(interfaceRegistry)
	cryptocodec.RegisterInterfaces(interfaceRegistry)
	ModuleBasics.RegisterLegacyAminoCodec(legacyAmino)

	txConfig := authtx.NewTxConfig(appCodec, authtx.DefaultSignModes)

	bApp := baseapp.NewBaseApp(Name, logger, db, txConfig.TxDecoder(), baseAppOptions...)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)
	bApp.SetTxEncoder(txConfig.TxEncoder())

	// sdk.NewKVStoreKeys was removed from the sdk package in v0.50 — create
	// the maps directly using storetypes so no helper wrapper is needed.
	tkeys := map[string]*storetypes.TransientStoreKey{
		paramstypes.TStoreKey: storetypes.NewTransientStoreKey(paramstypes.TStoreKey),
	}
	keys := map[string]*storetypes.KVStoreKey{
		authtypes.StoreKey:      storetypes.NewKVStoreKey(authtypes.StoreKey),
		banktypes.StoreKey:      storetypes.NewKVStoreKey(banktypes.StoreKey),
		stakingtypes.StoreKey:   storetypes.NewKVStoreKey(stakingtypes.StoreKey),
		paramstypes.StoreKey:    storetypes.NewKVStoreKey(paramstypes.StoreKey),
		consensustypes.StoreKey: storetypes.NewKVStoreKey(consensustypes.StoreKey),
		ageverifytypes.StoreKey: storetypes.NewKVStoreKey(ageverifytypes.StoreKey),
	}

	app := &App{
		BaseApp:           bApp,
		cdc:               appCodec,
		legacyAmino:       legacyAmino,
		interfaceRegistry: interfaceRegistry,
		txConfig:          txConfig,
		keys:              keys,
		tkeys:             tkeys,
	}

	app.ParamsKeeper = paramskeeper.NewKeeper(appCodec, legacyAmino, keys[paramstypes.StoreKey], tkeys[paramstypes.TStoreKey])
	app.ParamsKeeper.Subspace(authtypes.ModuleName)
	app.ParamsKeeper.Subspace(banktypes.ModuleName)
	app.ParamsKeeper.Subspace(stakingtypes.ModuleName)

	app.ConsensusKeeper = consensuskeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[consensustypes.StoreKey]),
		authtypes.NewModuleAddress("gov").String(),
		runtime.EventService{},
	)
	bApp.SetParamStore(app.ConsensusKeeper.ParamsStore)

	app.AccountKeeper = authkeeper.NewAccountKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[authtypes.StoreKey]),
		authtypes.ProtoBaseAccount,
		maccPerms,
		authcodec.NewBech32Codec(sdk.Bech32PrefixAccAddr),
		sdk.Bech32PrefixAccAddr,
		authtypes.NewModuleAddress("gov").String(),
	)

	app.BankKeeper = bankkeeper.NewBaseKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[banktypes.StoreKey]),
		app.AccountKeeper,
		BlockedAddresses(),
		authtypes.NewModuleAddress("gov").String(),
		logger,
	)

	app.StakingKeeper = stakingkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[stakingtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		authtypes.NewModuleAddress("gov").String(),
		authcodec.NewBech32Codec(sdk.Bech32PrefixValAddr),
		authcodec.NewBech32Codec(sdk.Bech32PrefixConsAddr),
	)

	app.AgeVerifyKeeper = ageverifykeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[ageverifytypes.StoreKey]),
		logger,
	)

	// genutil.NewAppModule takes 4 args in SDK v0.50 — the MessageValidator
	// lives in AppModuleBasic (ModuleBasics above), not in AppModule.
	app.mm = module.NewManager(
		genutil.NewAppModule(app.AccountKeeper, app.StakingKeeper, app.BaseApp, txConfig),
		auth.NewAppModule(appCodec, app.AccountKeeper, nil, app.GetSubspace(authtypes.ModuleName)),
		bank.NewAppModule(appCodec, app.BankKeeper, app.AccountKeeper, app.GetSubspace(banktypes.ModuleName)),
		staking.NewAppModule(appCodec, app.StakingKeeper, app.AccountKeeper, app.BankKeeper, app.GetSubspace(stakingtypes.ModuleName)),
		params.NewAppModule(app.ParamsKeeper),
		consensus.NewAppModule(appCodec, app.ConsensusKeeper),
		ageverifymodule.NewAppModule(app.AgeVerifyKeeper),
	)

	app.mm.SetOrderBeginBlockers(
		stakingtypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		consensustypes.ModuleName,
		ageverifytypes.ModuleName,
	)
	app.mm.SetOrderEndBlockers(
		stakingtypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		consensustypes.ModuleName,
		ageverifytypes.ModuleName,
	)
	app.mm.SetOrderInitGenesis(
		authtypes.ModuleName,
		banktypes.ModuleName,
		stakingtypes.ModuleName,
		genutiltypes.ModuleName,
		consensustypes.ModuleName,
		ageverifytypes.ModuleName,
	)
	app.mm.SetOrderExportGenesis(
		authtypes.ModuleName,
		banktypes.ModuleName,
		stakingtypes.ModuleName,
		genutiltypes.ModuleName,
		consensustypes.ModuleName,
		ageverifytypes.ModuleName,
	)

	app.configurator = module.NewConfigurator(appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	if err := app.mm.RegisterServices(app.configurator); err != nil {
		panic(err)
	}

	app.MountKVStores(keys)
	app.MountTransientStores(tkeys)
	app.SetInitChainer(app.InitChainer)
	app.SetBeginBlocker(app.BeginBlocker)
	app.SetEndBlocker(app.EndBlocker)

	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			panic(err)
		}
	}
	return app
}

func (app *App) GetSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := app.ParamsKeeper.GetSubspace(moduleName)
	return subspace
}

func (app *App) BeginBlocker(ctx sdk.Context) (sdk.BeginBlock, error) {
	return app.mm.BeginBlock(ctx)
}

func (app *App) EndBlocker(ctx sdk.Context) (sdk.EndBlock, error) {
	return app.mm.EndBlock(ctx)
}

// InitChainer uses *abci.RequestInitChain / *abci.ResponseInitChain from
// cometbft — sdk.InitChainRequest/Response don't exist in SDK v0.50.
func (app *App) InitChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var gs map[string]json.RawMessage
	if err := json.Unmarshal(req.AppStateBytes, &gs); err != nil {
		panic(err)
	}
	resp, err := app.mm.InitGenesis(ctx, app.cdc, gs)
	if err != nil {
		return nil, err
	}
	// Cosmos SDK / IAVL bug: an IAVL store that is completely empty (no keys
	// ever written) cannot serve CacheMultiStoreWithVersion queries because
	// GetRoot returns ErrVersionDoesNotExist for every version.  Writing a
	// single sentinel byte to each such store at genesis prevents this.
	ctx.KVStore(app.keys[paramstypes.StoreKey]).Set([]byte("initialized"), []byte{1})
	return resp, nil
}

func (app *App) ExportAppStateAndValidators(forZeroHeight bool, jailAllowedAddrs []string, modulesToExport []string) (servertypes.ExportedApp, error) {
	ctx := app.NewContextLegacy(true, cmtproto.Header{Height: app.LastBlockHeight()})
	gs, err := app.mm.ExportGenesis(ctx, app.cdc)
	if err != nil {
		return servertypes.ExportedApp{}, err
	}
	appState, err := json.MarshalIndent(gs, "", "  ")
	if err != nil {
		return servertypes.ExportedApp{}, err
	}
	validators, err := staking.WriteValidators(ctx, app.StakingKeeper)
	if err != nil {
		return servertypes.ExportedApp{}, err
	}
	return servertypes.ExportedApp{
		AppState:        appState,
		Validators:      validators,
		Height:          app.LastBlockHeight() + 1,
		ConsensusParams: app.GetConsensusParams(ctx),
	}, nil
}

func (app *App) RegisterAPIRoutes(_ *api.Server, _ config.APIConfig) {}

func (app *App) RegisterTendermintService(_ client.Context) {}

func (app *App) RegisterTxService(_ client.Context) {}

// RegisterNodeService is required by servertypes.Application in SDK v0.50.
func (app *App) RegisterNodeService(_ client.Context, _ config.Config) {}

func (app *App) DefaultGenesis() map[string]json.RawMessage {
	return ModuleBasics.DefaultGenesis(app.cdc)
}

func (app *App) AppCodec() codec.Codec                          { return app.cdc }
func (app *App) LegacyAmino() *codec.LegacyAmino                { return app.legacyAmino }
func (app *App) InterfaceRegistry() codectypes.InterfaceRegistry { return app.interfaceRegistry }
// TxConfig returns client.TxConfig — sdk.TxConfig does not exist in SDK v0.50.
func (app *App) TxConfig() client.TxConfig                       { return app.txConfig }
func (app *App) GetKey(storeKey string) *storetypes.KVStoreKey   { return app.keys[storeKey] }
func (app *App) GetStakingKeeper() *stakingkeeper.Keeper         { return app.StakingKeeper }

// authtypes.Staking (not authtypes.Staker) is the correct constant in SDK v0.50.
var maccPerms = map[string][]string{
	authtypes.FeeCollectorName:     nil,
	stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
	stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
}

func BlockedAddresses() map[string]bool {
	blocked := make(map[string]bool)
	for acc := range maccPerms {
		blocked[authtypes.NewModuleAddress(acc).String()] = true
	}
	return blocked
}
