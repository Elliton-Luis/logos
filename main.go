package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"logos/logos"
)

// ensureDirectoryStructure garante que a nova organização de pastas exista na primeira execução
func ensureDirectoryStructure() {
	directories := []string{"docs", "env", "logos"}
	for _, dir := range directories {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			_ = os.MkdirAll(dir, 0755)
			slog.Debug("Pasta criada automaticamente", "dir", dir)
		}
	}
}

func ensureDefaultPromptMD() {
	promptPath := filepath.Join("docs", "prompt.md")
	if _, err := os.Stat(promptPath); err == nil {
		return
	}
	
	exampleContent := `---
arquivos_obrigatorios: [index.html, js/script.js, css/style.css]
---

# Instrução Principal
Crie uma página moderna e limpa.
`
	_ = os.WriteFile(promptPath, []byte(exampleContent), 0644)
	slog.Info("Arquivo 'docs/prompt.md' de exemplo criado automaticamente!")
}

func formatThousands(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	result := strings.Join(parts, ".")
	if neg {
		result = "-" + result
	}
	return result
}

func main() {
	// Garante a estrutura perfeita de pastas logo no início
	ensureDirectoryStructure()
	logos.EnsureGitignore()
	ensureDefaultPromptMD()

	dryRun := flag.Bool("dry-run", false, "Shows changes without saving updates")
	verbose := flag.Bool("v", false, "Enables debug log output")
	providerFlag := flag.String("p", "", "AI Provider (groq, gemini)")
	modelFlag := flag.String("m", "", "AI Model identifier")
	
	// --- NOUVEAUX ATALHOS RÁPIDOS DE PROVEDOR ---
	groqShortcut := flag.Bool("groq", false, "Atalho rápido para usar o provedor GROQ")
	geminiShortcut := flag.Bool("gemini", false, "Atalho rápido para usar o provedor GEMINI")

	flag.Usage = func() {
		fmt.Println("Uso: logos [-p provider] [-m model] [-groq] [-gemini] <action> <target_files_or_dirs...> [instruction]")
		fmt.Println("\nExemplos:")
		fmt.Println("  logos -gemini feat main.py \"Função de soma de dois números\"")
		fmt.Println("  logos -groq fix index.html \"Corrija o bug do menu\"")
		fmt.Println("  logos feat cv.md template.md vaga.txt perfil.txt docs/prompt.md")
	}
	flag.Parse()

	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

	// Resolve a precedência das flags de provedor/modelo
	provider := *providerFlag
	model := *modelFlag

	if *geminiShortcut {
		provider = "gemini"
		if model == "" {
			model = "gemini-2.5-flash"
		}
	} else if *groqShortcut {
		provider = "groq"
		if model == "" {
			model = "llama-3.3-70b-versatile"
		}
	}

	cfg := logos.LoadConfig(provider, model, *verbose)

	var action string
	var targetPaths []string
	var instruction string

	args := flag.Args()

	// 1. MODO INTERATIVO
	if len(args) == 0 {
		fmt.Println("🚀 Logos CLI - Workspace Interactive Mode")
		fmt.Printf("🤖 Active AI: %s | Model: %s\n\n", strings.ToUpper(cfg.Provider), cfg.Model)

		action = strings.ToLower(logos.AskForInput("Action (feat, fix, refactor, doc, undo): "))
		pathsInput := logos.AskForInput("Target files or directories (separated by space): ")
		targetPaths = strings.Fields(pathsInput)

		if action != "undo" {
			instruction = logos.AskForInput("Type your instruction (or leave blank to use docs/prompt.md): ")
			if instruction == "" {
				if data, err := os.ReadFile(filepath.Join("docs", "prompt.md")); err == nil {
					instruction = strings.TrimSpace(string(data))
				}
			}
		}

	// 2. MODO CLI DINÂMICO
	} else {
		action = strings.ToLower(args[0])
		remainingArgs := args[1:]

		if len(remainingArgs) == 0 {
			slog.Error("You must provide at least one target file or directory.")
			os.Exit(1)
		}

		lastArg := remainingArgs[len(remainingArgs)-1]
		
		if _, err := os.Stat(lastArg); os.IsNotExist(err) {
			instruction = lastArg
			targetPaths = remainingArgs[:len(remainingArgs)-1]
		} else {
			targetPaths = remainingArgs
			if data, err := os.ReadFile(filepath.Join("docs", "prompt.md")); err == nil {
				slog.Info("Using default 'docs/prompt.md' for instruction context.")
				instruction = strings.TrimSpace(string(data))
			} else {
				slog.Error("No instruction string provided and 'docs/prompt.md' was not found.")
				os.Exit(1)
			}
		}
	}

	if len(targetPaths) == 0 {
		slog.Error("No target files found for processing.")
		os.Exit(1)
	}

	if err := logos.ValidateAPIKey(cfg); err != nil {
		slog.Error("Config Validation Failed", "reason", err.Error())
		os.Exit(1)
	}

	paths := logos.ResolveMetaPaths(targetPaths[0])

	if action == "undo" {
		logos.Rollback(paths)
		return
	}

	var allWorkspaceFiles []logos.FilePayload
	for _, path := range targetPaths {
		files, err := logos.ReadWorkspace(path)
		if err != nil {
			slog.Warn("Failed to read context path", "path", path, "error", err)
			continue
		}
		allWorkspaceFiles = append(allWorkspaceFiles, files...)
	}

	aiClient := logos.NewAIClient(cfg)
	cacheContext, _ := os.ReadFile(paths.Cache)

	systemPrompt := `Você é um engenheiro de software especialista em automação e geração de conteúdo técnico estruturado.
Você receberá múltiplos arquivos de contexto e deve retornar a resposta estritamente no formato JSON abaixo:
{
  "summary": "Resumo técnico sucinto das alterações feitas e quais arquivos foram modificados",
  "files": [
    {"path": "caminho/do/arquivo.ext", "content": "conteúdo completo atualizado do arquivo"}
  ]
}

⚠️ REGRAS DE ESCOPO E MINIMALISMO CRÍTICO:
1. Seja CIRÚRGICO e LITERAL. Faça única e exclusivamente o que o usuário pediu na instrução. Se ele pediu uma função de soma, crie uma função que soma dois parâmetros, não invente somas de listas complexas a menos que solicitado.
2. É ESTRITAMENTE PROIBIDO injetar códigos de exemplo, blocos "if __name__ == '__main__':", ou prints de teste no final do arquivo, a menos que o usuário tenha pedido explicitamente por exemplos ou testes.
3. Não adicione comentários prolixos, docstrings gigantescas ou explicações óbvias dentro do código, mantenha o código limpo, idiomático e direto ao ponto.
4. No array "files", inclua APENAS os arquivos que sofreram modificações.
5. Não altere arquivos estruturais do ecossistema do agente (como 'main.go') ou configurações ('env/.env').`

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Ação Solicitada: %s\n", action))
	sb.WriteString(fmt.Sprintf("Arquivos Alvo: %s\n", strings.Join(targetPaths, ", ")))
	sb.WriteString(fmt.Sprintf("Instrução de Execução:\n%s\n\n", instruction))

	if len(cacheContext) > 0 {
		sb.WriteString(fmt.Sprintf("Mapa Estrutural Prévio (Cache):\n%s\n\n", string(cacheContext)))
	}

	sb.WriteString("--- ARQUIVOS ENVIADOS AO WORKSPACE DE CONTEXTO ---\n")
	if len(allWorkspaceFiles) == 0 {
		sb.WriteString("(Nenhum arquivo encontrado no caminho especificado. Crie-os se necessário.)\n")
	} else {
		for _, f := range allWorkspaceFiles {
			sb.WriteString(fmt.Sprintf("📄 Arquivo: %s\n```\n%s\n```\n\n", f.Path, f.Content))
		}
	}

	userPromptText := sb.String()

	slog.Info(fmt.Sprintf("Consulting AI via %s (%s)...", cfg.Provider, cfg.Model))

	startTime := time.Now()
	rawAiResponse, tokens, err := aiClient.Generate(context.Background(), systemPrompt, userPromptText, 0.1)
	elapsed := time.Since(startTime)

	if err != nil {
		slog.Error("AI execution failure", "error", err)
		os.Exit(1)
	}

	parsedResponse, err := logos.ParseAIResponse(rawAiResponse)
	if err != nil {
		slog.Error("Failed parsing output structure", "error", err)
		os.Exit(1)
	}

	var securedFiles []logos.FilePayload
	for _, f := range parsedResponse.Files {
		cleanedPath := filepath.Clean(f.Path)
		
		if cleanedPath == "main.go" || strings.HasPrefix(cleanedPath, ".logos_meta") || strings.HasPrefix(cleanedPath, "env/") {
			slog.Error("🚨 Bloqueio de Segurança: A IA tentou modificar um arquivo restrito do sistema!", "path", f.Path)
			continue
		}
		securedFiles = append(securedFiles, f)
	}
	parsedResponse.Files = securedFiles

	if len(parsedResponse.Files) == 0 {
		slog.Error("Nenhuma alteração válida gerada pela IA dentro do escopo permitido.")
		os.Exit(1)
	}

	if err := logos.ShowWorkspaceDiff(paths, parsedResponse.Files); err != nil {
		slog.Warn("Could not display diff visualizer", "error", err)
	}

	fmt.Printf("\n✨ AI Technical Summary:\n%s\n\n", parsedResponse.Summary)
	fmt.Printf("🤖 Model: %s (%s) | 🔢 Tokens: %d | ⏱️  Time: %s\n\n",
		cfg.Model, cfg.Provider, tokens, elapsed.Round(time.Millisecond))

	if *dryRun {
		fmt.Println("\nMode --dry-run active. Changes were not written down.")
		return
	}

	if !logos.AskForConfirmation("\nApply workspace adjustments to the files above? (y/n): ") {
		fmt.Println("Cancelled.")
		return
	}

	if len(allWorkspaceFiles) > 0 {
		backupBytes, _ := json.Marshal(allWorkspaceFiles)
		_ = os.WriteFile(paths.Bak, backupBytes, 0600)
	}

	for _, f := range parsedResponse.Files {
		_ = os.MkdirAll(filepath.Dir(f.Path), 0755)
		if err := os.WriteFile(f.Path, []byte(f.Content), 0644); err != nil {
			slog.Error("Failed saving specific workspace file", "path", f.Path, "error", err)
		}
	}

	slog.Info("Workspace successfully updated!")

	totalTokens := logos.AppendProgress(paths.WorkspaceRoot, action, instruction, parsedResponse.Summary, cfg.Model, tokens, elapsed)
	_ = logos.UpdateCache(paths, parsedResponse.Files, aiClient, cfg)

	fmt.Printf("📊 Tokens usados (total): %s/%s\n\n", formatThousands(totalTokens), formatThousands(cfg.TokenBudget))
}