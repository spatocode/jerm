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

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy an application",
	Long:  "Deploy an application",
	Run: func(cmd *cobra.Command, args []string) {
		jerm.Verbose(cmd)

		cfg, err := jerm.Configure(jerm.DefaultConfigFile)
		if err != nil {
			log.PrintError(err)
			return
		}

		p, err := jerm.New(cfg)
		if err != nil {
			log.PrintError(err)
			return
		}

		platform, err := aws.NewLambda(cfg)
		if err != nil {
			log.PrintError(err)
			return
		}
		p.SetPlatform(platform)

		err = p.Deploy()
		if err != nil {
			log.PrintError(err)
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
