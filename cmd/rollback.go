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
)

// rollbackCmd represents the rollback command
var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rolls back to the previous revision of the deployment",
	Long:  "Rolls back to the previous revision of the deployment",
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
		p.Rollback()
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
