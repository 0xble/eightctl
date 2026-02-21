package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/steipete/eightctl/internal/client"
)

var presenceCmd = &cobra.Command{
	Use:   "presence",
	Short: "Check if user is in bed",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthFields(); err != nil {
			return err
		}
		cl := client.New(viper.GetString("email"), viper.GetString("password"), viper.GetString("user_id"), viper.GetString("client_id"), viper.GetString("client_secret"))
		present, err := cl.GetPresence(context.Background())
		if err != nil {
			if client.IsEndpointUnavailable(err) {
				fmt.Println("present: unknown (endpoint unavailable)")
				return nil
			}
			return err
		}
		fmt.Printf("present: %v\n", present)
		return nil
	},
}
