package logos

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Provider string
	APIKey   string
	Model    string
	Verbose  bool
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

	return Config{
		Provider: provider,
		Model:    model,
		APIKey:   apiKey,
		Verbose:  verbose,
	}
}

func ValidateAPIKey(cfg Config) error {
	key := strings.TrimSpace(cfg.APIKey)
	if key == "" {
		return fmt.Errorf("a chave de API para o slowedor '%s' não foi encontrada no seu arquivo .env", strings.ToUpper(cfg.Provider))
	}

	if cfg.Provider == "groq" {
		if !strings.HasPrefix(key, "gsk_") {
			return fmt.Errorf("chave Groq inválida! Deve começar com 'gsk_'. Verifique o GROQ_API_KEY no .env")
		}
	} else if cfg.Provider == "gemini" {
		if !strings.HasPrefix(key, "AIzaSy") && !strings.HasPrefix(key, "AQ.") {
			return fmt.Errorf("sua chave do Gemini parece incorreta! Chaves do Google AI Studio começam com 'AIzaSy' ou 'AQ.'")
		}
	}

	return nil
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

func EnsureDotEnvTemplate() {
	if _, err := os.Stat(".env"); err == nil {
		return
	}

	template := `# LOGOS CLI - CONFIGURAÇÃO (APENAS GROQ E GEMINI)
LOGOS_DEFAULT_PROVIDER=gemini
LOGOS_DEFAULT_MODEL=gemini-2.5-flash

# Cole suas chaves de API correspondentes abaixo (SEM ASPAS):
GROQ_API_KEY=
GEMINI_API_KEY=
`
	_ = os.WriteFile(".env", []byte(template), 0644)
	slog.Info("Arquivo '.env' criado automaticamente! Configure suas chaves.")
}

func EnsureGitignore() {
	if _, err := os.Stat(".gitignore"); err == nil {
		return
	}
	content := ".env\nprogress.md\n.logos_meta/\n"
	_ = os.WriteFile(".gitignore", []byte(content), 0644)
}