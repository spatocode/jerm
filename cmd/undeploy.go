/*
Copyright © 2023 Ekene Izukanne <ekeneizukanne@gmail.com>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// undeployCmd represents the undeploy command
var undeployCmd = &cobra.Command{
	Use:   "undeploy",
	Short: "Undeploy a deployed application",
	Long:  "Undeploy a deployed application",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("undeploy called")
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
