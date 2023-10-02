/*
Copyright © 2023 Ekene Izukanne <ekeneizukanne@gmail.com>
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/spatocode/jerm"
	"github.com/spatocode/jerm/cloud/aws"
	"github.com/spatocode/jerm/internal/log"
	"github.com/spatocode/jerm/internal/utils"
)

// undeployCmd represents the undeploy command
var undeployCmd = &cobra.Command{
	Use:   "undeploy",
	Short: "Undeploy a deployed application",
	Long:  "Undeploy a deployed application",
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
		log.PrintWarn("Are you sure you want to undeploy? [y/n]")
		ans, err := utils.ReadPromptInput("", os.Stdin)
		if err != nil {
			log.PrintError(err.Error())
			return
		}
		if ans != "y" {
			return
		}
		p.Undeploy()
	},
}

func init() {
	rootCmd.AddCommand(undeployCmd)
}
