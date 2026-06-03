package cmd

import (
	"io"
	"os"

	cmtcfg "github.com/cometbft/cometbft/config"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/gogoproto/proto"
	"cosmossdk.io/x/tx/signing"
	"github.com/spf13/cobra"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"

	"ageverify/app"
	ageverifycli "ageverify/x/ageverify/client/cli"
)

// NewRootCmd creates the root `ageverifyd` command.
func NewRootCmd() *cobra.Command {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("cosmos", "cosmospub")
	cfg.SetBech32PrefixForValidator("cosmosvaloper", "cosmosvaloperpub")
	cfg.SetBech32PrefixForConsensusNode("cosmosvalcons", "cosmosvalconspub")
	cfg.Seal()

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

	app.ModuleBasics.RegisterInterfaces(interfaceRegistry)
	cryptocodec.RegisterInterfaces(interfaceRegistry)
	app.ModuleBasics.RegisterLegacyAminoCodec(legacyAmino)

	txConfig := authtx.NewTxConfig(appCodec, authtx.DefaultSignModes)

	initClientCtx := client.Context{}.
		WithInput(os.Stdin).
		WithHomeDir(app.DefaultNodeHome).
		WithViper("AGEVERIFY").
		WithCodec(appCodec).
		WithInterfaceRegistry(interfaceRegistry).
		WithLegacyAmino(legacyAmino).
		WithTxConfig(txConfig).
		WithAccountRetriever(authtypes.AccountRetriever{})

	rootCmd := &cobra.Command{
		Use:   "ageverifyd",
		Short: "ageverify — ZK Age Verification Chain",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
			if err != nil {
				return err
			}
			initClientCtx, err = config.ReadFromClientConfig(initClientCtx)
			if err != nil {
				return err
			}
			if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
				return err
			}
			// Only intercept server configs for the 'start' command. Other commands
			// (init, keys, gentx, query, tx …) don't need the CometBFT / app config
			// setup and will fail if we call InterceptConfigsPreRunHandler before
			// the node home has been fully initialised.
			if cmd.Name() == "start" {
				return server.InterceptConfigsPreRunHandler(cmd, "", nil, cmtcfg.DefaultConfig())
			}
			return nil
		},
	}

	rootCmd.AddCommand(
		genutilcli.InitCmd(app.ModuleBasics, app.DefaultNodeHome),
		genutilcli.Commands(txConfig, app.ModuleBasics, app.DefaultNodeHome),
		server.StartCmd(newApp, app.DefaultNodeHome),
		server.ExportCmd(exportApp, app.DefaultNodeHome),
		keys.Commands(),
		queryCmd(),
		txCmd(),
	)

	return rootCmd
}

func queryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	cmd.AddCommand(
		rpc.ValidatorCommand(),
		authcmd.QueryTxsByEventsCmd(),
		authcmd.QueryTxCmd(),
		ageverifycli.GetQueryCmd(),
	)
	return cmd
}

func txCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	cmd.AddCommand(
		authcmd.GetSignCommand(),
		authcmd.GetMultiSignCommand(),
		authcmd.GetValidateSignaturesCommand(),
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
		ageverifycli.GetTxCmd(),
	)
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func newApp(logger log.Logger, db dbm.DB, traceStore io.Writer, appOpts servertypes.AppOptions) servertypes.Application {
	baseAppOpts := server.DefaultBaseappOptions(appOpts)
	return app.NewApp(logger, db, traceStore, true, appOpts, baseAppOpts...)
}

func exportApp(logger log.Logger, db dbm.DB, traceStore io.Writer, height int64, forZeroHeight bool, jailAllowedAddrs []string, appOpts servertypes.AppOptions, modulesToExport []string) (servertypes.ExportedApp, error) {
	a := app.NewApp(logger, db, traceStore, height == -1, appOpts)
	if height != -1 {
		if err := a.LoadVersion(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	}
	return a.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}
