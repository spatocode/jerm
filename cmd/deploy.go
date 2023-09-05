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
	"github.com/spatocode/jerm/internal/log"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy an application",
	Long:  "Deploy an application",
	Run: func(cmd *cobra.Command, args []string) {
		// verbose, _ := cmd.Flags().GetBool("verbose")

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

		if len(args) == 1 && strings.ToLower(args[0]) == "aws" {
			platform, err := aws.NewLambda(config)
			if err != nil {
				log.PrintError(err.Error())
				return
			}
			p.SetPlatform(platform)
			p.Deploy()
		} else {
			log.PrintError("Unknown arg. Expected a cloud platform [aws]")
		}
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)

	// deployCmd.Flags().BoolP("production", "p", false, "Sets production stage")
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deployCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// deployCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
