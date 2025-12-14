package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type CommandFunc func(args []string) error

var commands map[string]CommandFunc

func main() {
	commands = map[string]CommandFunc{
		"exit": runExit,
		"echo": runEcho,
		"type": runType,
	}
	for {
		fmt.Print("$ ")
		command, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
			os.Exit(-1)
		}
		processInput(command)
	}

}

func runExit(args []string) error {
	if len(args) == 0 {
		os.Exit(0)
	}
	exitCode, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error converting command to int", err)
		os.Exit(-1)
	}
	os.Exit(exitCode)
	return nil
}

func runEcho(args []string) error {
	argsString := strings.Join(args, " ")
	fmt.Println(argsString)
	return nil
}

func runType(args []string) error {
	if len(args) == 0 {
		return nil
	}
	for _, arg := range args {
		_, exists := commands[arg]
		if exists {
			fmt.Println(arg + " is a shell builtin")
			return nil
		}
		path, err := exec.LookPath(arg)
		if err != nil {
			fmt.Println(arg + ": not found")
			return nil
		}
		fmt.Println(arg + " is " + path)
	}
	return nil
}

func processInput(command string) {
	args := strings.Fields(command)
	if len(args) == 0 {
		return
	}
	commandArg, ok := commands[args[0]]
	if ok {
		err := commandArg(args[1:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error executing command", err)
			return
		}
		return
	} else {
		path, err := exec.LookPath(args[0])
		if err != nil {
			fmt.Println(args[0] + ": command not found")
			return
		}
		output, err := exec.Command(path, strings.Join(args[1:], " ")).Output()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error running command: ", err)
		}
		fmt.Println(output)
	}
}
