package cmd

import (
	"io"
	"os"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/spf13/cobra"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"

	"ageverify/app"
)

// NewRootCmd creates the root `ageverifyd` command.
func NewRootCmd() *cobra.Command {
	// Set bech32 prefixes (cosmos standard)
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("cosmos", "cosmospub")
	cfg.SetBech32PrefixForValidator("cosmosvaloper", "cosmosvaloperpub")
	cfg.SetBech32PrefixForConsensusNode("cosmosvalcons", "cosmosvalconspub")
	cfg.Seal()

	initClientCtx := client.Context{}.
		WithInput(os.Stdin).
		WithHomeDir(app.DefaultNodeHome).
		WithViper("AGEVERIFY")

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
			return server.InterceptConfigsPreRunHandler(cmd, "", nil, nil)
		},
	}

	// Persistent flags
	rootCmd.PersistentFlags().String(flags.FlagHome, app.DefaultNodeHome, "The application home directory")
	rootCmd.PersistentFlags().String(flags.FlagLogLevel, "info", "The logging level (trace|debug|info|warn|error|fatal|panic)")
	rootCmd.PersistentFlags().String(flags.FlagLogFormat, "plain", "The logging format (json|plain)")

	// Sub-commands
	rootCmd.AddCommand(
		genutilcli.InitCmd(app.ModuleBasics, app.DefaultNodeHome),
		genutilcli.CollectGenTxsCmd(banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome, genutiltypes.DefaultMessageValidator, nil),
		genutilcli.MigrateGenesisCmd(genutiltypes.MigrationMap{}),
		genutilcli.GenTxCmd(app.ModuleBasics, nil, nil, banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome, nil),
		genutilcli.ValidateGenesisCmd(app.ModuleBasics),
		genutilcli.AddGenesisAccountCmd(app.DefaultNodeHome),
		server.StartCmd(newApp, app.DefaultNodeHome),
		server.ExportCmd(exportApp, app.DefaultNodeHome),
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
		app.ModuleBasics.GetQueryCmd(),
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
		app.ModuleBasics.GetTxCmd(),
	)
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func newApp(logger log.Logger, db dbm.DB, traceStore io.Writer, appOpts servertypes.AppOptions) servertypes.Application {
	return app.NewApp(logger, db, traceStore, true, appOpts)
}

func exportApp(logger log.Logger, db dbm.DB, traceStore io.Writer, height int64, forZeroHeight bool, jailAllowedAddrs []string, appOpts servertypes.AppOptions, modulesToExport []string) (servertypes.ExportedApp, error) {
	a := app.NewApp(logger, db, traceStore, height == -1, appOpts)
	if height != -1 {
		if err := a.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	}
	return a.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}

// Suppress unused import warning
var _ = types.AccountRetriever{}
