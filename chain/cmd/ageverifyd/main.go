package main

import (
	"os"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/server"
	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"ageverify/app"
)

func main() {
	// Set bech32 prefixes for ageverify chain
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("cosmos", "cosmospub")
	cfg.Seal()

	rootCmd := NewRootCmd()
	if err := svrcmd.Execute(rootCmd, "AGEVERIFY", app.DefaultNodeHome); err != nil {
		log.NewLogger(os.Stderr).Error("root command error", "err", err)
		os.Exit(1)
	}
}

// NewRootCmd constructs the root command for ageverifyd.
func NewRootCmd() *cobra.Command {
	encodingConfig := app.ModuleBasics.DefaultGenesis
	initClientCtx := client.Context{}.
		WithCodec(nil).
		WithInterfaceRegistry(nil).
		WithTxConfig(nil).
		WithLegacyAmino(nil).
		WithInput(os.Stdin).
		WithAccountRetriever(authtypes.AccountRetriever{}).
		WithHomeDir(app.DefaultNodeHome).
		WithViper("AGEVERIFY")

	rootCmd := &cobra.Command{
		Use:   "ageverifyd",
		Short: "ageverify ZK Age Verification Chain",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())
			return nil
		},
	}

	// Add standard cosmos sdk commands
	rootCmd.AddCommand(
		server.StartCmd(newApp, app.DefaultNodeHome),
		genutilcli.InitCmd(app.ModuleBasics, app.DefaultNodeHome),
		genutilcli.CollectGenTxsCmd(banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome, genutiltypes.DefaultMessageValidator, nil),
		genutilcli.GenTxCmd(app.ModuleBasics, nil, stakingtypes.DefaultParams(), banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome, nil),
		genutilcli.ValidateGenesisCmd(app.ModuleBasics),
		genutilcli.AddGenesisAccountCmd(app.DefaultNodeHome),
		keys.Commands(),
		status.NewStatusCommand(),
		queryCommand(),
		txCommand(),
	)

	_ = encodingConfig
	_ = initClientCtx
	return rootCmd
}

func newApp(logger log.Logger, db dbm.DB, traceStore io.Writer, appOpts servertypes.AppOptions) servertypes.Application {
	return app.NewApp(logger, db, traceStore, true, appOpts)
}

func queryCommand() *cobra.Command {
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
		server.QueryBlockCmd(),
		authcmd.QueryTxsByEventsCmd(),
		authcmd.QueryTxCmd(),
		app.ModuleBasics.GetQueryCmd(),
	)
	return cmd
}

func txCommand() *cobra.Command {
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
		authcmd.GetMultiSignBatchCmd(),
		authcmd.GetValidateSignaturesCommand(),
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
		app.ModuleBasics.GetTxCmd(),
	)
	return cmd
}
