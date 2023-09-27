/*
Copyright © 2023 Ekene Izukanne <ekeneizukanne@gmail.com>
*/
package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/spatocode/jerm"
	"github.com/spatocode/jerm/cloud/aws"
	"github.com/spatocode/jerm/config"
	"github.com/spatocode/jerm/internal/log"
	"github.com/spatocode/jerm/internal/utils"
)

// manageCmd represents the manage command
var manageCmd = &cobra.Command{
	Use:   "manage",
	Short: "Manage runs a Django management command",
	Long:  "Manage runs a Django management command",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.PrintError("manage expects a Django management command")
			return
		}

		cfg, err := jerm.Configure(jerm.DefaultConfigFile)
		if err != nil {
			log.PrintError(err.Error())
			return
		}

		platform, err := aws.NewLambda(cfg)
		if err != nil {
			log.PrintError(err.Error())
			return
		}

		runtime := config.NewPythonRuntime(utils.Command())
		python := runtime.(*config.Python)
		if !strings.HasPrefix(cfg.Platform.Runtime, "python") || !python.IsDjango() {
			log.PrintError("manage command is for Django projects only")
			return
		}

		p, err := jerm.New(cfg)
		if err != nil {
			log.PrintError(err.Error())
			return
		}

		p.SetPlatform(platform)
		p.Invoke(strings.Join(args, " "))
	},
}

func init() {
	rootCmd.AddCommand(manageCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// manageCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// manageCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
