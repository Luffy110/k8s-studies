package cmd

import (
	"githelper/pkg/exec"

	"github.com/spf13/cobra"
)

var (
	rebase bool
)

func init() {
	rootCmd.AddCommand(pullCmd)

	pullCmd.Flags().BoolVarP(&rebase, "rebase", "r", false, "--rebase")
}

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "pull code",
	Long:  `pull code from remote repositroy`,
	Run: func(cmd *cobra.Command, args []string) {
		runPull(args)
	},
}

func runPull(args []string) {
	argus := []string{"pull"}
	argus = append(argus, args...)
	if rebase {
		argus = append(argus, "--rebase")
	}
	exec.DoExecCommand(argus)
}
