package logos

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	APIKey    string
	Model     string
	Verbose   bool
	TargetDir string // Novo campo para rastrear a pasta alvo de testes/produção
}

func LoadDotEnv() {
	file, err := os.Open(".env")
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if os.Getenv(key) == "" {
			os.Setenv(key, strings.Trim(strings.TrimSpace(parts[1]), `"'`))
		}
	}
}

// EnsureGitignore garante a criação do .gitignore de forma concisa
func EnsureGitignore() {
	if _, err := os.Stat(".gitignore"); err == nil {
		return // Já existe
	}
	content := ".env\nprogress.md\n*.bak\n*.cache\n*.cache.hash\n"
	_ = os.WriteFile(".gitignore", []byte(content), 0644)
}