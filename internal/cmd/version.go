package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "0.2.0-0xble.0.1.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}
