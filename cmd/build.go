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

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "build an application",
	Long:  "build an application",
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
		p.Build()
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
