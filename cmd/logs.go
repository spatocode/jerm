/*
Copyright © 2023 Ekene Izukanne <ekeneizukanne@gmail.com>
*/
package cmd

import (
	"errors"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/spatocode/jerm"
	"github.com/spatocode/jerm/cloud/aws"
	"github.com/spatocode/jerm/internal/utils"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show deployment logs",
	Long:  "Show deployment logs",
	Run: func(cmd *cobra.Command, args []string) {
		config, err := jerm.ReadConfig(jerm.DefaultConfigFile)
		if err != nil {
			var pErr *os.PathError
			if !errors.As(err, &pErr) {
				utils.LogError(err.Error())
			}
		}

		p, err := jerm.New(config)
		if err != nil {
			utils.LogError(err.Error())
			return
		}

		if len(args) == 1 && strings.ToLower(args[0]) == "aws" {
			platform, err := aws.NewLambda(config)
			if err != nil {
				utils.LogError(err.Error())
				return
			}
			p.SetPlatform(platform)
			p.Logs()
		} else {
			utils.LogError("Unknown arg. Expected a cloud platform [aws]")
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// statusCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// statusCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
