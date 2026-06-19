package logos

import (
	"fmt"
	"os"
	"os/exec"
)

func ShowWorkspaceDiff(paths MetaPaths, proposedFiles []FilePayload) error {
	fmt.Println("\n--- 🔍 VISUALIZADOR DE ALTERAÇÕES (DIFF) ---")

	for _, file := range proposedFiles {
		tmpNew, err := os.CreateTemp("", "logos_diff_new")
		if err != nil {
			return fmt.Errorf("failed to create temporary file for diff: %w", err)
		}
		defer os.Remove(tmpNew.Name())

		if _, err := tmpNew.WriteString(file.Content); err != nil {
			tmpNew.Close()
			return fmt.Errorf("failed to write to temporary file: %w", err)
		}
		tmpNew.Close()

		origPathForDiff := file.Path
		if _, err := os.Stat(file.Path); os.IsNotExist(err) {
			origPathForDiff = os.DevNull
		}

		fmt.Printf("📄 Arquivo: %s\n", file.Path)
		cmd := exec.Command("diff", "-u", "--color=always", origPathForDiff, tmpNew.Name())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		fmt.Println("--------------------------------------------")
	}

	return nil
}