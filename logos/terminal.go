package logos

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func PrintUsage() {
	fmt.Println("Logos CLI - High-Performance AI Code Editor")
	fmt.Println("\nStandard Usage:")
	fmt.Println("  logos [-m model] [-v] [--dry-run] <action> <file> [\"instruction\"]")
	fmt.Println("\nIf called without arguments, starts Interactive Mode.")
	fmt.Println("\nEditing Actions:")
	fmt.Println("  feat     : Create a file or add a new feature")
	fmt.Println("  fix      : Fix an existing bug or issue")
	fmt.Println("  refactor : Optimize code structure and performance")
	fmt.Println("  doc      : Add documentation or comments")
	fmt.Println("\nMaintenance Actions:")
	fmt.Println("  cache    : Force AI to generate structural map (.cache)")
	fmt.Println("  undo     : Revert file to the last local backup (.bak)")
}

func AskForConfirmation(prompt string) bool {
	fmt.Print(prompt)
	resp, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	r := strings.ToLower(strings.TrimSpace(resp))
	return r == "y" || r == "yes"
}

func AskForInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func ShowDiff(paths MetaPaths, newContent string) error {
	tmpNew, err := os.CreateTemp("", "logos_diff_new")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for diff: %w", err)
	}
	defer os.Remove(tmpNew.Name())

	if _, err := tmpNew.WriteString(newContent); err != nil {
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}
	tmpNew.Close()

	origPathForDiff := paths.Source
	if _, err := os.Stat(paths.Source); os.IsNotExist(err) {
		origPathForDiff = os.DevNull
	}

	fmt.Println("\n--- Diff ---")
	cmd := exec.Command("diff", "-u", "--color=always", origPathForDiff, tmpNew.Name())
	cmd.Stdout = os.Stdout
	cmd.Run()
	fmt.Println("------------\n")
	return nil
}