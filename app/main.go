package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	//"path/filepath"
)

type CommandFunc func(args []string) error

var commands map[string]CommandFunc

var directory string
var dirError error

func main() {
	commands = map[string]CommandFunc{
		"exit": runExit,
		"echo": runEcho,
		"type": runType,
		"pwd":  runPwd,
		"cd":   runCd,
	}

	directory, dirError = os.Getwd()
	if dirError != nil {
		fmt.Fprintln(os.Stderr, "error reading current directory ", dirError)
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

func runPwd(args []string) error {
	if len(args) > 1 {
		fmt.Println("pwd: too many arguments")
		return nil
	}
	fmt.Println(directory)
	return nil
}

func runCd(args []string) error {
	if len(args) > 1 {
		fmt.Println("cd: too many arguments")
		return nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error reading current directory ", dirError)
		return dirError
	}
	if len(args) == 0 {
		directory = homeDir
		return nil
	}
	destination := args[0]
	destinationParts := strings.Split(destination, "/")
	if len(destinationParts[len(destinationParts)-1]) == 0 {
		destinationParts = destinationParts[:len(destinationParts)-1]
	}

	switch destinationParts[0] {
	case "":
		if _, err := os.Stat(destination); errors.Is(err, os.ErrNotExist) {
			fmt.Println("cd: " + destination + ": No such file or directory")
			return nil
		}
		directory = destination
		err := os.Chdir(directory)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cd: error changing directory", err)
			return nil
		}
		return nil
	case "~":
		finalPath := homeDir + strings.Join(destinationParts[1:], "/")
		if _, err := os.Stat(finalPath); errors.Is(err, os.ErrNotExist) {
			fmt.Println("cd: " + finalPath + ": No such file or directory")
			return nil
		}
		directory = finalPath

		err := os.Chdir(directory)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cd: error changing directory", err)
			return nil
		}
		return nil
	default:
		tempDir := directory + "/" + destination
		info, e := os.Stat(tempDir)
		if e != nil {
			fmt.Println("cd: " + destination + ": no such file or directory")
			return nil
		}
		if !(info.IsDir()) {
			fmt.Println("cd: not a directory: " + destination)
			return nil
		}
		currentDirectoryParts := strings.Split(directory, "/")
		var tempArray []string = currentDirectoryParts
		for _, dir := range destinationParts {
			switch dir {
			case "..":
				if len(tempArray) == 1 {
					continue
				}
				tempArray = tempArray[:len(tempArray)-1]
				break
			case ".":
				break
			default:
				tempArray = append(tempArray, dir)
				break
			}
		}
		directory = strings.Join(tempArray, "/")
		err := os.Chdir(directory)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cd: error changing directory", err)
			return nil
		}
		break
	}

	return nil

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
	}

	cmd := exec.Command(args[0], args[1:]...)
	var out strings.Builder
	cmd.Stdout = &out
	e := cmd.Run()
	if e != nil {
		fmt.Println(args[0] + ": command not found")
		return
	}
	fmt.Print(out.String())
}
