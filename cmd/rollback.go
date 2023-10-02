/*
Copyright © 2023 Ekene Izukanne <ekeneizukanne@gmail.com>
*/
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/spatocode/jerm"
	"github.com/spatocode/jerm/cloud/aws"
	"github.com/spatocode/jerm/internal/log"
)

// rollbackCmd represents the rollback command
var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rolls back to the previous versions of the deployment",
	Long:  "Rolls back to the previous versions of the deployment",
	Run: func(cmd *cobra.Command, args []string) {
		steps, _ := cmd.Flags().GetInt("steps")
		jerm.Verbose(cmd)

		cfg, err := jerm.Configure(jerm.DefaultConfigFile)
		if err != nil {
			log.PrintError(err.Error())
			return
		}

		p, err := jerm.New(cfg)
		if err != nil {
			log.PrintError(err.Error())
			return
		}

		platform, err := aws.NewLambda(cfg)
		if err != nil {
			log.PrintError(err.Error())
			return
		}
		p.SetPlatform(platform)
		p.Rollback(steps)
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)

	rollbackCmd.Flags().IntP("steps", "s", 1, "Number of previous versions")
}
