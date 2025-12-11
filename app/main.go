package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func exit(splitCommand []string) string {
	if len(splitCommand) == 2 {
		exitCode, err := strconv.Atoi(splitCommand[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "error reading exit code", err)
			os.Exit(-1)
		}
		os.Exit(exitCode)
	} else if len(splitCommand) == 1 {
		os.Exit(0)
	}
	return "error: too many arguments"
}

func processInput(inputString string) string {
	var splitCommand = strings.Split(inputString[:len(inputString)-1], " ")
	switch splitCommand[0] {
	case "exit":
		return exit(splitCommand)
	default:
		return splitCommand[0] + ": command not found"
	}

}

func main() {

	for {
		fmt.Print("$ ")
		command, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
			os.Exit(-1)
		}
		fmt.Println(processInput(command))
	}

}
