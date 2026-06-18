package logos

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MetaPaths guarda os caminhos mapeados para o arquivo e seus metadados
type MetaPaths struct {
	Source string // O arquivo de código real (ex: testes/script.go)
	Bak    string // O backup (ex: testes/script-go/.bak)
	Cache  string // O cache (ex: testes/script-go/.cache)
	Hash   string // O hash (ex: testes/script-go/.hash)
}

// ResolveMetaPaths calcula os caminhos e cria os diretórios necessários
func ResolveMetaPaths(inputPath string) MetaPaths {
	dir := filepath.Dir(inputPath)
	base := filepath.Base(inputPath)

	// Substitui o ponto por hífen para criar o nome da pasta de isolamento (ex: script.go -> script-go)
	folderName := strings.ReplaceAll(base, ".", "-")
	metaDir := filepath.Join(dir, folderName)

	// Cria os diretórios caso não existam
	_ = os.MkdirAll(dir, 0755)
	_ = os.MkdirAll(metaDir, 0755)

	return MetaPaths{
		Source: inputPath,
		Bak:    filepath.Join(metaDir, ".bak"),
		Cache:  filepath.Join(metaDir, ".cache"),
		Hash:   filepath.Join(metaDir, ".hash"),
	}
}

func ReadOrCreateFile(paths MetaPaths, action string) (string, error) {
	data, err := os.ReadFile(paths.Source)
	if err == nil {
		return string(data), nil
	}
	if os.IsNotExist(err) && action == "feat" {
		if !AskForConfirmation(fmt.Sprintf("File '%s' does not exist. Do you want to create it from scratch? (y/n): ", paths.Source)) {
			fmt.Println("Operation cancelled.")
			os.Exit(0)
		}
		return "", nil
	}
	return "", err
}

func ReadFileIfExists(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func Rollback(paths MetaPaths) {
	data, err := os.ReadFile(paths.Bak)
	if err != nil {
		slog.Error("No backup file found in isolated folder", "file", paths.Bak)
		return
	}
	if err := os.WriteFile(paths.Source, data, 0644); err != nil {
		slog.Error("Failed to revert the file", "error", err)
		return
	}
	slog.Info("Rollback completed successfully!", "file", paths.Source)
	AppendProgress(paths.Source, "undo", "Reversion", "Restored from local isolated backup.", "N/A", 0)
}

func AppendProgress(filePath, action, instruction, aiSummary, modelName string, tokens int) {
	const progressFile = "progress.md"

	existing, _ := os.ReadFile(progressFile)
	lineNumber := strings.Count(string(existing), "### ") + 1
	timestamp := time.Now().Format("02/01/2006 15:04")

	tokenInfo := "N/A"
	if tokens > 0 {
		tokenInfo = fmt.Sprintf("%d", tokens)
	}

	entry := fmt.Sprintf("\n### %d. `%s` (%s) - %s\n> **Instruction:** %s\n> **Model:** %s\n> **Tokens:** %s\n\n**Technical Summary:**\n%s\n\n---\n",
		lineNumber, filePath, action, timestamp, instruction, modelName, tokenInfo, aiSummary)

	f, err := os.OpenFile(progressFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Warn("Failed to record progress log", "error", err)
		return
	}
	defer f.Close()

	f.WriteString(entry)
}

func CleanMarkdown(text string) string {
	text = strings.TrimSpace(text)

	if strings.HasPrefix(text, "```") {
		firstNewLine := strings.Index(text, "\n")
		if firstNewLine != -1 {
			text = text[firstNewLine+1:]
		}
	}

	if strings.HasSuffix(text, "```") {
		text = text[:len(text)-3]
	}

	return strings.TrimSpace(text)
}