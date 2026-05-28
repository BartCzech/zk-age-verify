package cli

import (
	"context"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"ageverify/x/ageverify/types"
)

// GetQueryCmd returns the query commands for this module.
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "ageverify query subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	cmd.AddCommand(CmdQueryVerificationStatus())
	return cmd
}

// CmdQueryVerificationStatus queries whether an address is age-verified.
func CmdQueryVerificationStatus() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verification-status [address]",
		Short: "Query the verification status for an address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.VerificationStatus(context.Background(), &types.QueryVerificationStatusRequest{
				Address: args[0],
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
