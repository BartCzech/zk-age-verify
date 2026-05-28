package app

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	dbm "github.com/cosmos/cosmos-db"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"

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
	txConfig          sdk.TxConfig

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
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	appCodec := codec.NewProtoCodec(interfaceRegistry)
	legacyAmino := codec.NewLegacyAmino()

	ModuleBasics.RegisterInterfaces(interfaceRegistry)
	ModuleBasics.RegisterLegacyAminoCodec(legacyAmino)

	// DefaultSignModes = DIRECT + AMINO_JSON. The demo always passes
	// --sign-mode amino-json because MsgSubmitAgeProof was written by
	// hand (no protoc), so SIGN_MODE_DIRECT signer resolution won't work.
	txConfig := authtx.NewTxConfig(appCodec, authtx.DefaultSignModes)

	bApp := baseapp.NewBaseApp(Name, logger, db, txConfig.TxDecoder(), baseAppOptions...)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)
	bApp.SetTxEncoder(txConfig.TxEncoder())

	tkeys := sdk.NewTransientStoreKeys(paramstypes.TStoreKey)
	keys := sdk.NewKVStoreKeys(
		authtypes.StoreKey,
		banktypes.StoreKey,
		stakingtypes.StoreKey,
		paramstypes.StoreKey,
		consensustypes.StoreKey,
		ageverifytypes.StoreKey,
	)

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

	app.mm = module.NewManager(
		genutil.NewAppModule(app.AccountKeeper, app.StakingKeeper, app.BaseApp, txConfig, genutiltypes.DefaultMessageValidator),
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

func (app *App) InitChainer(ctx sdk.Context, req *sdk.InitChainRequest) (*sdk.InitChainResponse, error) {
	var gs map[string]json.RawMessage
	if err := json.Unmarshal(req.AppStateBytes, &gs); err != nil {
		panic(err)
	}
	return app.mm.InitGenesis(ctx, app.cdc, gs)
}

func (app *App) ExportAppStateAndValidators(forZeroHeight bool, jailAllowedAddrs []string, modulesToExport []string) (servertypes.ExportedApp, error) {
	ctx := app.NewContextLegacy(true, sdk.Header{Height: app.LastBlockHeight()})
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

func (app *App) DefaultGenesis() map[string]json.RawMessage {
	return app.mm.DefaultGenesis(app.cdc)
}

func (app *App) AppCodec() codec.Codec                          { return app.cdc }
func (app *App) LegacyAmino() *codec.LegacyAmino                { return app.legacyAmino }
func (app *App) InterfaceRegistry() codectypes.InterfaceRegistry { return app.interfaceRegistry }
func (app *App) TxConfig() sdk.TxConfig                         { return app.txConfig }
func (app *App) GetKey(storeKey string) *storetypes.KVStoreKey   { return app.keys[storeKey] }
func (app *App) GetStakingKeeper() *stakingkeeper.Keeper         { return app.StakingKeeper }

var maccPerms = map[string][]string{
	authtypes.FeeCollectorName:     nil,
	stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staker},
	stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staker},
}

func BlockedAddresses() map[string]bool {
	blocked := make(map[string]bool)
	for acc := range maccPerms {
		blocked[authtypes.NewModuleAddress(acc).String()] = true
	}
	return blocked
}
