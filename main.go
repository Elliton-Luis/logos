package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"logos/logos"
)

func ensureDefaultPromptMD() {
	if _, err := os.Stat("prompt.md"); err == nil {
		return
	}
	exampleContent := `---
arquivos_obrigatorios: [index.html, js/script.js, css/style.css]
---

# Instrução Principal
Crie uma página moderna e limpa.

# Especificações de Arquivos

## index.html
- Deve conter a estrutura base HTML5 explicando boas práticas de prompts.
`
	_ = os.WriteFile("prompt.md", []byte(exampleContent), 0644)
	slog.Info("Arquivo 'prompt.md' de exemplo criado automaticamente na raiz!")
}

func main() {
	logos.EnsureGitignore()
	ensureDefaultPromptMD()

	dryRun := flag.Bool("dry-run", false, "Shows changes without saving updates")
	verbose := flag.Bool("v", false, "Enables debug log output")
	providerFlag := flag.String("p", "", "AI Provider (groq, gemini)")
	modelFlag := flag.String("m", "", "AI Model identifier")

	flag.Usage = logos.PrintUsage
	flag.Parse()

	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

	cfg := logos.LoadConfig(*providerFlag, *modelFlag, *verbose)

	var action, targetPath, instruction string

	// 1. MODO INTERATIVO
	if len(flag.Args()) == 0 {
		fmt.Println("🚀 Logos CLI - Workspace Interactive Mode")
		fmt.Printf("🤖 Active AI: %s | Model: %s\n\n", strings.ToUpper(cfg.Provider), cfg.Model)
		
		if !logos.AskForConfirmation("Use current AI settings? (y/n): ") {
			cfg.Provider = strings.ToLower(logos.AskForInput("Change Provider to (groq, gemini): "))
			cfg.Model = logos.AskForInput("Change Model identifier: ")
			cfg = logos.LoadConfig(cfg.Provider, cfg.Model, *verbose)
		}

		action = strings.ToLower(logos.AskForInput("\nAction (feat, fix, refactor, doc, undo, cache): "))
		targetPath = logos.AskForInput("Target file or directory: ")

		if action != "undo" && action != "cache" {
			if _, err := os.Stat("prompt.md"); err == nil {
				if logos.AskForConfirmation("Found 'prompt.md'. Do you want to use it? (y/n): ") {
					data, _ := os.ReadFile("prompt.md")
					instruction = strings.TrimSpace(string(data))
				}
			}
			if instruction == "" {
				instruction = logos.AskForInput("Type your instruction: ")
			}
		}

	// 2. MODO ARQUIVO ÚNICO (Ex: go run . prompt.md)
	} else if len(flag.Args()) == 1 {
		argFile := flag.Arg(0)
		if _, err := os.Stat(argFile); err == nil {
			data, err := os.ReadFile(argFile)
			if err != nil {
				slog.Error("Failed to read prompt file", "path", argFile, "error", err)
				os.Exit(1)
			}
			slog.Info("Running in Single Prompt File Mode", "file", argFile)
			
			action = "feat" 
			targetPath = "." 
			instruction = strings.TrimSpace(string(data))
		} else {
			slog.Error("Single argument provided but it is not a valid file path", "arg", argFile)
			os.Exit(1)
		}

	// 3. MODO TRADICIONAL
	} else {
		action = strings.ToLower(flag.Arg(0))
		targetPath = flag.Arg(1)
		
		if len(flag.Args()) >= 3 {
			argPrompt := flag.Arg(2)
			if _, err := os.Stat(argPrompt); err == nil {
				data, err := os.ReadFile(argPrompt)
				if err == nil {
					slog.Info("Reading prompt instruction from file", "path", argPrompt)
					instruction = strings.TrimSpace(string(data))
				} else {
					instruction = argPrompt
				}
			} else {
				instruction = argPrompt
			}
		} else if action != "undo" && action != "cache" {
			if data, err := os.ReadFile("prompt.md"); err == nil {
				slog.Info("No instruction argument provided. Using default 'prompt.md'")
				instruction = strings.TrimSpace(string(data))
			} else {
				slog.Error("No instruction provided and 'prompt.md' missing.")
				os.Exit(1)
			}
		}
	}

	if err := logos.ValidateAPIKey(cfg); err != nil {
		slog.Error("Config Validation Failed", "reason", err.Error())
		os.Exit(1)
	}

	paths := logos.ResolveMetaPaths(targetPath)

	if action == "undo" {
		logos.Rollback(paths)
		return
	}

	workspaceFiles, err := logos.ReadWorkspace(paths.WorkspaceRoot)
	if err != nil {
		slog.Error("Failed to read workspace state", "error", err)
		os.Exit(1)
	}

	aiClient := logos.NewAIClient(cfg)

	if action == "cache" {
		if err := logos.UpdateCache(paths, workspaceFiles, aiClient, cfg); err != nil {
			slog.Error("Failed to update structure cache", "error", err)
			os.Exit(1)
		}
		return
	}

	cacheContext, _ := os.ReadFile(paths.Cache)

	// 🔥 ADAPTADO: O systemPrompt agora força o cumprimento estrito dos parâmetros do arquivo
	systemPrompt := `Você é um engenheiro de software experiente. Você recebe o contexto de um workspace e uma instrução estruturada.
DIRETRIZES DE ESCOPO:
1. Se a instrução possuir uma lista de arquivos obrigatórios ou mapeamento de caminhos (ex: index.html, js/script.js), você DEVE criar exatamente esses arquivos nos caminhos informados.
2. Se um arquivo foi listado no cabeçalho ou metadados, mas não detalhado na especificação do texto, use seu conhecimento para criá-lo de forma completa e profissional para complementar o projeto.

Sua resposta DEVE obrigatoriamente ser um formato JSON válido com a exata estrutura abaixo:
{
  "summary": "Resumo técnico sucinto das alterações feitas",
  "files": [
    {"path": "caminho/do/arquivo.ext", "content": "conteúdo completo atualizado do arquivo"}
  ]
}`

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Ação Solicitada: %s\n", action))
	sb.WriteString(fmt.Sprintf("Instrução do Usuário:\n%s\n\n", instruction))
	
	if len(cacheContext) > 0 {
		sb.WriteString(fmt.Sprintf("Mapa Estrutural Prévio (Cache):\n%s\n\n", string(cacheContext)))
	}
	
	sb.WriteString("--- ARQUIVOS ATUAIS NO WORKSPACE ---\n")
	if len(workspaceFiles) == 0 {
		sb.WriteString("(O workspace está vazio, crie a árvore de arquivos respeitando os parâmetros da instrução)\n")
	} else {
		for _, f := range workspaceFiles {
			sb.WriteString(fmt.Sprintf("📄 Arquivo: %s\n```\n%s\n
```\n\n", f.Path, f.Content))
		}
	}

	userPromptText := sb.String()

	slog.Info(fmt.Sprintf("Consulting AI via %s (%s)...", cfg.Provider, cfg.Model))

	startTime := time.Now()
	rawAiResponse, tokens, err := aiClient.Generate(context.Background(), systemPrompt, userPromptText, 0.2)
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

	if err := logos.ShowWorkspaceDiff(paths, parsedResponse.Files); err != nil {
		slog.Warn("Could not display diff visualizer", "error", err)
	}

	fmt.Printf("\nAI Technical Summary:\n%s\n\n", parsedResponse.Summary)
	fmt.Printf("Model: %s (%s) | 🔢 Tokens: %d | ⏱️  Time: %s\n\n",
		cfg.Model, cfg.Provider, tokens, elapsed.Round(time.Millisecond))

	if *dryRun {
		fmt.Println("\nMode --dry-run active. Changes were not written down.")
		return
	}

	if !logos.AskForConfirmation("\nApply workspace adjustments? (y/n): ") {
		fmt.Println("Cancelled.")
		return
	}

	if len(workspaceFiles) > 0 {
		backupBytes, _ := json.Marshal(workspaceFiles)
		_ = os.WriteFile(paths.Bak, backupBytes, 0600)
	}

	for _, f := range parsedResponse.Files {
		_ = os.MkdirAll(filepath.Dir(f.Path), 0755)
		if err := os.WriteFile(f.Path, []byte(f.Content), 0644); err != nil {
			slog.Error("Failed saving specific workspace file", "path", f.Path, "error", err)
		}
	}

	slog.Info("Workspace successfully updated!")
	logos.AppendProgress(paths.WorkspaceRoot, action, instruction, parsedResponse.Summary, cfg.Model, tokens)
	_ = logos.UpdateCache(paths, parsedResponse.Files, aiClient, cfg)
}