package main

import (
	"os"

	"cosmossdk.io/log"
	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"

	"ageverify/app"
	"ageverify/cmd/ageverifyd/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, "AGEVERIFY", app.DefaultNodeHome); err != nil {
		log.NewLogger(os.Stderr).Error("fatal error", "err", err)
		os.Exit(1)
	}
}
