package main

import (
	"fmt"
	"os"

	"github.com/evanboyle/halloumi/pkg/orchestrator"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewHalloumiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "halloumi",
		Short: "halloumi is a tool that melds go application and infrastructure",
		Long:  `halloumi is a tool that melds go application and infrastructure`,
		Args:  cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			apps, err := orchestrator.DryRun(args[0])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			err = orchestrator.Deploy(apps)
			if err != nil {
				fmt.Println(errors.Wrap(err, "failed to deploy program"))
				os.Exit(1)
			}
		},
	}
	return cmd
}
