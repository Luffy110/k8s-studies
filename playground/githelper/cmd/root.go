package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "githelper",
	Short: "A git helper",
	Long:  `A git helper cli for convenience to use git command`,
	Run: func(cmd *cobra.Command, args []string) {
		// invoke help func to show user how to use this tool
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
