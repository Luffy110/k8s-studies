package exec

import (
	"fmt"
	"os/exec"
)

func DoExecCommand(args []string) {
	command := exec.Command("git", args...)

	fmt.Println(command.String())

	output, err := command.Output()
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(string(output))
	}
}
