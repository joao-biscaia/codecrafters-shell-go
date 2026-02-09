package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/chzyer/readline"
)

type ParsedCommand struct {
	name   string
	args   []string
	stdout io.Writer
	stderr io.Writer
}

type CommandFunc func(sh *Shell, ctx *ExecContext, args []string) error

type ExecContext struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

type Shell struct {
	workingDirectory string
	builtinCommands  map[string]CommandFunc
}

type AutoCompleter struct {
	completer readline.AutoCompleter
	tabCount  int
	lastInput string
}

func (auto *AutoCompleter) Do(line []rune, pos int) ([][]rune, int) {
	newLine, length := auto.completer.Do(line, pos)

	currentInput := string(line)
	if currentInput != auto.lastInput {
		auto.tabCount = 0
		auto.lastInput = currentInput
	}

	if len(newLine) == 1 {
		auto.tabCount = 0
		fmt.Print(string(newLine[0]))
		return newLine, length
	}

	if len(newLine) > 1 {
		auto.tabCount++
		if auto.tabCount == 1 {
			fmt.Print("\x07")
			return [][]rune{}, 0
		}
		fmt.Print("\n")
		for _, match := range newLine {
			prefix := string(line[:pos])
			fmt.Print(prefix + strings.TrimRight(string(match), " ") + "  ")
		}
		fmt.Print("\n")
		fmt.Print("$ " + string(line))
		auto.tabCount = 0
		return [][]rune{}, 0
	}

	if len(newLine) == 0 {
		fmt.Print("\x07")
	}

	return newLine, length
}

func getPathExecutables() [][]rune {
	path := os.Getenv("PATH")
	pathDirectories := strings.Split(path, string(os.PathListSeparator))
	var pathExecutables [][]rune

	for _, pathDirectory := range pathDirectories {
		if filesInDir, err := os.ReadDir(pathDirectory); !errors.Is(err, os.ErrNotExist) {
			for _, file := range filesInDir {
				if !file.IsDir() {
					pathExecutables = append(pathExecutables, []rune(file.Name()))
				}
			}
		}
	}

	sort.Slice(pathExecutables, func(i, j int) bool { return string(pathExecutables[i]) < string(pathExecutables[j]) })

	return pathExecutables
}

func createPathExecsItems(pathExecs [][]rune) *readline.PrefixCompleter {
	var items []readline.PrefixCompleterInterface

	items = append(items,
		readline.PcItem("echo"),
		readline.PcItem("exit"),
		readline.PcItem("cd"),
		readline.PcItem("pwd"),
		readline.PcItem("type"),
	)

	cmdArray := []string{"echo", "exit", "cd", "pwd", "type"}

	for _, pathExec := range pathExecs {
		if !slices.Contains(cmdArray, string(pathExec)) {
			items = append(items, readline.PcItem(string(pathExec)))
		}
	}

	return readline.NewPrefixCompleter(items...)
}

func main() {
	dir, _ := os.Getwd()

	shell := &Shell{workingDirectory: dir,
		builtinCommands: map[string]CommandFunc{
			"pwd":  (*Shell).runPwd,
			"cd":   (*Shell).runCd,
			"exit": (*Shell).runExit,
			"echo": (*Shell).runEcho,
			"type": (*Shell).runType,
		}}

	shell.execute()

}

func (sh *Shell) execute() {

	pathExecs := getPathExecutables()
	completer := createPathExecsItems(pathExecs)

	customCompleter := &AutoCompleter{completer: completer}
	for {
		fmt.Print("$ ")

		context := &ExecContext{
			stdin:  os.Stdin,
			stdout: os.Stdout,
			stderr: os.Stderr,
		}

		rlInstance, _ := readline.NewEx(&readline.Config{
			Prompt:       "$ ",
			AutoComplete: customCompleter,
		})

		command, err := rlInstance.Readline()

		if err != nil {
			errorString := "Error reading input: " + err.Error()
			_, err := context.stderr.Write([]byte(errorString))
			if err != nil {
				os.Exit(-1)
			}
			os.Exit(-1)
		}

		sh.processInput(context, command)
	}
}

func (sh *Shell) parseCommand(context *ExecContext, args []string) ParsedCommand {

	parsedArgs := ParsedCommand{
		name:   args[0],
		args:   args,
		stdout: context.stdout,
		stderr: context.stderr,
	}
	var fileOut string

	var redirected = false
	for idx, arg := range args {
		if ((arg == ">") || (arg == "1>")) && idx < (len(args)-1) {
			if !redirected {
				parsedArgs.args = args[:idx]
				redirected = true
			}
			fileOut = args[idx+1]
			file, _ := os.Create(fileOut)
			parsedArgs.stdout = file
		} else if (arg == "2>") && idx < (len(args)-1) {
			file, _ := os.Create(args[idx+1])
			parsedArgs.stderr = file
			if !redirected {
				parsedArgs.args = args[:idx]
				redirected = true
			}
		} else if ((arg == ">>") || (arg == "1>>")) && idx < (len(args)-1) {
			file, _ := os.OpenFile(args[idx+1], os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
			parsedArgs.stdout = file
			if !redirected {
				parsedArgs.args = args[:idx]
				redirected = true
			}
		} else if ((arg == ">>") || (arg == "2>>")) && idx < (len(args)-1) {
			file, _ := os.OpenFile(args[idx+1], os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
			parsedArgs.stderr = file
			if !redirected {
				parsedArgs.args = args[:idx]
				redirected = true
			}
		}
	}

	return parsedArgs

}

func (sh *Shell) processInput(context *ExecContext, command string) {

	var args = splitArgs(command)
	if len(args) == 0 {
		return
	}
	parsedArgs := sh.parseCommand(context, args)
	args = parsedArgs.args
	context.stdout = parsedArgs.stdout
	context.stderr = parsedArgs.stderr

	commandArg, ok := sh.builtinCommands[args[0]]
	if ok {
		err := commandArg(sh, context, args[1:])
		if err != nil {
			_, err := context.stderr.Write([]byte("Error executing command\n"))
			if err != nil {
				os.Exit(-1)
				return
			}
			return
		}
		return
	}
	cmd := exec.Command(parsedArgs.name, args[1:]...)
	var out strings.Builder
	var cmdErr strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &cmdErr

	err := cmd.Run()

	if out.Len() > 0 {
		fmt.Fprint(context.stdout, out.String())
	}

	if cmdErr.Len() > 0 {
		fmt.Fprint(context.stderr, cmdErr.String())
	}

	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			fmt.Fprintf(context.stderr, "%s: command not found\n", parsedArgs.name)
		}
	}

}

func (sh *Shell) runPwd(context *ExecContext, args []string) error {
	if len(args) > 1 {
		context.stderr.Write([]byte("pwd: too many arguments\n"))
		return nil
	}
	_, err := context.stdout.Write([]byte(sh.workingDirectory + "\n"))
	if err != nil {
		return err
	}
	return nil
}

func (sh *Shell) runCd(context *ExecContext, args []string) error {
	if len(args) > 1 {
		fmt.Fprintln(context.stderr, "cd: too many arguments")
		return nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(context.stderr, "error reading current directory ")
		return nil
	}
	if len(args) == 0 {
		sh.workingDirectory = homeDir
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
			fmt.Fprintln(context.stderr, "cd: "+destination+": No such file or directory")
			return nil
		}
		sh.workingDirectory = destination
		err := os.Chdir(sh.workingDirectory)
		if err != nil {
			fmt.Fprintln(context.stderr, "cd: error changing directory")
			return nil
		}
		return nil
	case "~":
		finalPath := homeDir + strings.Join(destinationParts[1:], "/")
		if _, err := os.Stat(finalPath); errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(context.stderr, "cd: "+finalPath+": No such file or directory")
			return nil
		}
		sh.workingDirectory = finalPath

		err := os.Chdir(sh.workingDirectory)
		if err != nil {
			fmt.Fprintln(context.stderr, "cd: error changing directory")
			return nil
		}
		return nil
	default:
		tempDir := sh.workingDirectory + "/" + destination
		info, e := os.Stat(tempDir)
		if e != nil {
			fmt.Fprintln(context.stderr, "cd: "+destination+": no such file or directory")
			return nil
		}
		if !(info.IsDir()) {
			fmt.Fprintln(context.stderr, "cd: not a directory: "+destination)
			return nil
		}
		currentDirectoryParts := strings.Split(sh.workingDirectory, "/")
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
		sh.workingDirectory = strings.Join(tempArray, "/")
		err := os.Chdir(sh.workingDirectory)
		if err != nil {
			fmt.Fprintln(context.stderr, "cd: error changing directory")
			return nil
		}
		break
	}

	return nil
}

func (sh *Shell) runExit(context *ExecContext, args []string) error {
	if len(args) == 0 {
		os.Exit(0)
	}
	exitCode, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintln(context.stderr, "Error exiting program with code"+err.Error())
		os.Exit(-1)
	}
	os.Exit(exitCode)
	return nil
}

func (sh *Shell) runEcho(context *ExecContext, args []string) error {
	argsString := strings.Join(args, " ")
	fmt.Fprintln(context.stdout, argsString)
	return nil
}

func (sh *Shell) runType(context *ExecContext, args []string) error {
	if len(args) == 0 {
		return nil
	}
	for _, arg := range args {
		_, exists := sh.builtinCommands[arg]
		if exists {
			fmt.Fprintln(context.stdout, arg+" is a shell builtin")
			return nil
		}
		path, err := exec.LookPath(arg)
		if err != nil {
			fmt.Fprintln(context.stderr, arg+": not found")
			return nil
		}
		fmt.Fprintln(context.stdout, arg+" is "+path)
	}
	return nil
}

func splitArgs(input string) []string {
	var args []string
	var currentArg strings.Builder

	inSingleQuote := false
	inDoubleQuote := false
	isEscaped := false

	//Escape
	for _, r := range input {
		if isEscaped {
			if inDoubleQuote {
				switch r {
				case '$', '"', '\\', '\n':
					currentArg.WriteRune(r)
				default:
					currentArg.WriteRune('\\')
					currentArg.WriteRune(r)
				}
			} else {
				currentArg.WriteRune(r)
			}
			isEscaped = false
			continue
		}
		if r == '\\' {
			if !inSingleQuote {
				isEscaped = true
				continue
			}
		}
		//Single quotes
		if r == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		//Double quotes
		if r == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		//Space
		if r == ' ' || r == '\t' {
			if inSingleQuote || inDoubleQuote {
				currentArg.WriteRune(r)
			} else {

				if currentArg.Len() > 0 {
					args = append(args, currentArg.String())
					currentArg.Reset()
				}
			}
			continue

		}

		currentArg.WriteRune(r)
	}

	if currentArg.Len() > 0 {
		args = append(args, currentArg.String())
	}

	return args
}
