/*
Copyright © 2023 Ekene Izukanne <ekeneizukanne@gmail.com>
*/
package cmd

import (
	"errors"
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
		config, err := jerm.ReadConfig(jerm.DefaultConfigFile)
		if err != nil {
			var pErr *os.PathError
			if !errors.As(err, &pErr) {
				log.PrintError(err.Error())
			}
		}

		p, err := jerm.New(config)
		if err != nil {
			log.PrintError(err.Error())
			return
		}

		platform, err := aws.NewLambda(config)
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

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// undeployCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// undeployCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
