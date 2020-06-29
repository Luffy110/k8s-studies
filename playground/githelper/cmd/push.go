package cmd

import (
	"fmt"
	"githelper/pkg/exec"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var (
	branch           string
	reviewer         string
	reviewerFromFile string
)

func init() {
	rootCmd.AddCommand(pushCmd)

	pushCmd.Flags().StringVarP(&branch, "branch", "b", "master", "--branch dev")
	pushCmd.Flags().StringVarP(&reviewer, "reviewer", "r", "", "--reviewer xxxx.gmail.com")
	pushCmd.Flags().StringVarP(&reviewerFromFile, "reviewers", "", "", "--reviewers reviewers config file name")
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "push your code",
	Long:  `push your code to remote repository`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(args)
		runPush(args)
	},
}

func runPush(args []string) {
	argus := []string{"push"}
	argus = append(argus, args...)

	reviewers := []string{}
	if reviewer != "" {
		reviewers = append(reviewers, reviewer)
	}

	if reviewerFromFile != "" {
		b, err := ioutil.ReadFile(reviewerFromFile)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		} else {
			//remove blank lines
			s := regexp.MustCompile(`[\t\r\n]+`).ReplaceAllString(strings.TrimSpace(string(b)), "\n")

			reviewers = append(reviewers, strings.Split(s, "\n")...)
		}
	}

	str := branch
	if len(reviewers) > 0 {
		str += "%"
	}
	for i, r := range reviewers {
		str += "r="
		if i == len(reviewers)-1 {
			str += r
		} else {
			str += r + ","
		}
	}

	argus = append(argus, str)
	exec.DoExecCommand(argus)
}
