package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version retains the fork suffix while tracking the upstream release base.
var Version = "0.2.5-0xble.0.1.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}
