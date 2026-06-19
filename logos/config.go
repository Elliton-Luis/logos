package logos

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Provider    string
	APIKey      string
	Model       string
	Verbose     bool
	TokenBudget int
}

func LoadConfig(providerFlag, modelFlag string, verbose bool) Config {
	EnsureDotEnvTemplate()
	LoadDotEnv()

	provider := providerFlag
	if provider == "" {
		provider = os.Getenv("LOGOS_DEFAULT_PROVIDER")
	}
	if provider == "" {
		provider = "groq"
	}
	provider = strings.ToLower(provider)
	if provider != "groq" && provider != "gemini" {
		provider = "groq"
	}

	model := modelFlag
	if model == "" {
		model = os.Getenv("LOGOS_DEFAULT_MODEL")
	}
	if model == "" {
		if provider == "gemini" {
			model = "gemini-2.5-flash"
		} else {
			model = "llama-3.3-70b-versatile"
		}
	}

	apiKey := os.Getenv(strings.ToUpper(provider) + "_API_KEY")
	if apiKey == "" && provider == "groq" {
		apiKey = os.Getenv("GROQ_API_KEY")
	}

	tokenBudget := 100000
	if v := os.Getenv("LOGOS_TOKEN_BUDGET"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			tokenBudget = n
		}
	}

	return Config{
		Provider:    provider,
		Model:       model,
		APIKey:      apiKey,
		Verbose:     verbose,
		TokenBudget: tokenBudget,
	}
}

func ValidateAPIKey(cfg Config) error {
	key := strings.TrimSpace(cfg.APIKey)
	if key == "" {
		return fmt.Errorf("a chave de API para o provedor '%s' não foi encontrada no seu arquivo env/.env", strings.ToUpper(cfg.Provider))
	}

	if cfg.Provider == "groq" {
		if !strings.HasPrefix(key, "gsk_") {
			return fmt.Errorf("chave Groq inválida! Deve começar com 'gsk_'. Verifique o GROQ_API_KEY no env/.env")
		}
	} else if cfg.Provider == "gemini" {
		if !strings.HasPrefix(key, "AIzaSy") && !strings.HasPrefix(key, "AQ.") {
			return fmt.Errorf("sua chave do Gemini parece incorreta! Chaves do Google AI Studio começam com 'AIzaSy' ou 'AQ.'")
		}
	}

	return nil
}

func LoadDotEnv() {
	file, err := os.Open(filepath.Join("env", ".env"))
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

func EnsureDotEnvTemplate() {
	envPath := filepath.Join("env", ".env")
	if _, err := os.Stat(envPath); err == nil {
		return
	}

	_ = os.MkdirAll("env", 0755)

	template := `# LOGOS CLI - CONFIGURAÇÃO (APENAS GROQ E GEMINI)
LOGOS_DEFAULT_PROVIDER=gemini
LOGOS_DEFAULT_MODEL=gemini-2.5-flash
LOGOS_TOKEN_BUDGET=100000

# Cole suas chaves de API correspondentes abaixo (SEM ASPAS):
GROQ_API_KEY=
GEMINI_API_KEY=
`
	_ = os.WriteFile(envPath, []byte(template), 0644)
	slog.Info("Arquivo 'env/.env' criado automaticamente! Configure suas chaves.")
}

func EnsureGitignore() {
	// Se já existe, vamos ler para garantir que as regras críticas estão lá dentro
	data, err := os.ReadFile(".gitignore")
	content := ""
	if err == nil {
		content = string(data)
	}

	rules := []string{
		"env/",
		".logos_meta/",
		"docs/",
	}

	modified := false
	for _, rule := range rules {
		if !strings.Contains(content, rule) {
			if content != "" && !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			content += rule + "\n"
			modified = true
		}
	}

	if modified || err != nil {
		_ = os.WriteFile(".gitignore", []byte(content), 0644)
		slog.Info("Arquivo '.gitignore' atualizado com as regras de proteção!")
	}
}