package logos

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type MetaPaths struct {
	WorkspaceRoot string
	Bak           string
	Cache         string
	Hash          string
}

func ResolveMetaPaths(targetPath string) MetaPaths {
	metaDir := filepath.Join(".logos_meta", strings.ReplaceAll(filepath.Clean(targetPath), string(filepath.Separator), "_"))
	_ = os.MkdirAll(metaDir, 0755)

	return MetaPaths{
		WorkspaceRoot: targetPath,
		Bak:           filepath.Join(metaDir, ".bak.json"),
		Cache:         filepath.Join(metaDir, ".cache"),
		Hash:          filepath.Join(metaDir, ".hash"),
	}
}

func ReadWorkspace(target string) ([]FilePayload, error) {
	var files []FilePayload
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return files, nil
		}
		return nil, err
	}

	if !info.IsDir() {
		data, err := os.ReadFile(target)
		if err != nil {
			return nil, err
		}
		files = append(files, FilePayload{Path: target, Content: string(data)})
		return files, nil
	}

	err = filepath.Walk(target, func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fileInfo.IsDir() && (strings.HasPrefix(fileInfo.Name(), ".") || fileInfo.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !fileInfo.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".go" || ext == ".html" || ext == ".js" || ext == ".css" || ext == ".json" || ext == ".md" || ext == ".txt" {
				data, err := os.ReadFile(path)
				if err == nil {
					files = append(files, FilePayload{Path: path, Content: string(data)})
				}
			}
		}
		return nil
	})

	return files, err
}

func Rollback(paths MetaPaths) {
	data, err := os.ReadFile(paths.Bak)
	if err != nil {
		slog.Error("Nenhum ponto de restauração em lote encontrado.", "file", paths.Bak)
		return
	}

	var backups []FilePayload
	if err := json.Unmarshal(data, &backups); err != nil {
		slog.Error("Falha ao ler dados do backup", "error", err)
		return
	}

	for _, bk := range backups {
		_ = os.MkdirAll(filepath.Dir(bk.Path), 0755)
		_ = os.WriteFile(bk.Path, []byte(bk.Content), 0644)
	}
	slog.Info("Rollback do espaço de trabalho executado com sucesso!")
}

// AppendProgress grava uma nova entrada no docs/progress.md e retorna o total
// acumulado de tokens usados (incluindo esta execução).
func AppendProgress(target, action, instruction, aiSummary, modelName string, tokens int, elapsed time.Duration) int {
	const progressFile = "docs/progress.md"
	
	// Garante que a pasta docs exista antes de tentar ler/escrever
	_ = os.MkdirAll("docs", 0755)
	
	existing, _ := os.ReadFile(progressFile)
	lineNumber := strings.Count(string(existing), "### ") + 1
	timestamp := time.Now().Format("02/01/2006 15:04")

	tokenInfo := "N/A"
	if tokens > 0 {
		tokenInfo = fmt.Sprintf("%d", tokens)
	}

	entry := fmt.Sprintf("\n### %d. `%s` (%s) - %s\n> **Instruction:** %s\n> **Model:** %s\n> **Tokens:** %s\n> **Time:** %s\n\n**Technical Summary:**\n%s\n\n---\n",
		lineNumber, target, action, timestamp, instruction, modelName, tokenInfo, elapsed.Round(time.Millisecond), aiSummary)

	f, err := os.OpenFile(progressFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		_, _ = f.WriteString(entry)
	}

	return sumTokens(string(existing)) + tokens
}

func sumTokens(content string) int {
	total := 0
	for _, line := range strings.Split(content, "\n") {
		if idx := strings.Index(line, "**Tokens:**"); idx != -1 {
			numStr := strings.TrimSpace(line[idx+len("**Tokens:**"):])
			if n, err := strconv.Atoi(numStr); err == nil {
				total += n
			}
		}
	}
	return total
}

func AskForInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func AskForConfirmation(prompt string) bool {
	res := AskForInput(prompt)
	res = strings.ToLower(res)
	return res == "y" || res == "yes" || res == "s" || res == "sim"
}

func PrintUsage() {
	fmt.Println("Uso: logos [-p groq|gemini] [-m model] <action> <target_file_or_folder> [instruction]")
}