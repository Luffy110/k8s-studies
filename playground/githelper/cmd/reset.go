package cmd

import (
	"githelper/pkg/exec"

	"github.com/spf13/cobra"
)

var (
	hard string
)

func init() {
	rootCmd.AddCommand(resetCmd)

	resetCmd.Flags().StringVarP(&hard, "hard", "", "", "--hard HEAD^")
}

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "reset local commit",
	Long:  `reset local commit`,
	Run: func(cmd *cobra.Command, args []string) {
		runReset(args)
	},
}

func runReset(args []string) {
	argus := []string{"reset"}
	argus = append(argus, args...)
	if hard != "" {
		argus = append(args, "--hard")
		argus = append(argus, hard)
	}
	exec.DoExecCommand(argus)
}
