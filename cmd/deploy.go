/*
Copyright © 2023 Ekene Izukanne <ekeneizukanne@gmail.com>
*/
package cmd

import (
	"strings"

	"github.com/spatocode/bulaba/project"
	"github.com/spatocode/bulaba/utils"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy an application",
	Long:  "Deploy an application",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 1 && strings.ToLower(args[0]) == "aws" {
			lambda := project.NewProject()
			lambda.DeployAWS()
		} else {
			utils.BulabaException("Unknown arg. Expected a cloud platform [aws]")
		}
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deployCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// deployCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
