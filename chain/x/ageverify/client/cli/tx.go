package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"ageverify/x/ageverify/types"
)

// GetTxCmd returns the transaction commands for this module.
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "ageverify transaction subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	cmd.AddCommand(CmdSubmitAgeProof())
	return cmd
}

// CmdSubmitAgeProof submits a ZK age proof to the chain.
func CmdSubmitAgeProof() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-age-proof [proof] [public-witness] [current-date]",
		Short: "Submit a ZK proof that proves age >= 18",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgSubmitAgeProof{
				Creator:       clientCtx.GetFromAddress().String(),
				Proof:         args[0],
				PublicWitness: args[1],
				CurrentDate:   args[2],
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
