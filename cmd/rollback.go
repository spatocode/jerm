/*
Copyright © 2023 Ekene Izukanne <ekeneizukanne@gmail.com>
*/
package cmd

import (
	"strings"

	"github.com/spatocode/bulaba/cloud"
	"github.com/spatocode/bulaba/project"
	"github.com/spatocode/bulaba/utils"
	"github.com/spf13/cobra"
)

// rollbackCmd represents the rollback command
var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollsback to the previous revision of the deployment",
	Long:  "Rollsback to the previous revision of the deployment",
	Run: func(cmd *cobra.Command, args []string) {
		p := project.LoadProject()
		config := p.JSONToStruct()
		if len(args) == 1 && strings.ToLower(args[0]) == "aws" {
			lambda := cloud.LoadLambda(config)
			lambda.Rollback()
		} else {
			utils.BulabaException("Unknown arg. Expected a cloud platform [aws]")
		}
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// rollbackCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// rollbackCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
