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

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show deployment logs",
	Long:  "Show deployment logs",
	Run: func(cmd *cobra.Command, args []string) {
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
		p.Logs()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
