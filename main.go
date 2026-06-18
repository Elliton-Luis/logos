package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"logos/logos"
)

func main() {
	logos.LoadDotEnv()
	logos.EnsureGitignore()

	dryRun := flag.Bool("dry-run", false, "Shows diff without saving updates")
	verbose := flag.Bool("v", false, "Enables debug log output")
	modelFlag := flag.String("m", "llama-3.3-70b-versatile", "AI Model identifier")

	flag.Usage = logos.PrintUsage
	flag.Parse()

	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	cfg := logos.Config{
		APIKey:  os.Getenv("GROQ_API_KEY"),
		Model:   *modelFlag,
		Verbose: *verbose,
	}

	if cfg.APIKey == "" {
		slog.Error("GROQ_API_KEY environment variable not set in system or .env file")
		os.Exit(1)
	}

	var action, filePath, instruction string

	if len(flag.Args()) == 0 {
		fmt.Println("🚀 Logos CLI - Interactive Mode")
		action = strings.ToLower(logos.AskForInput("Action (feat, fix, refactor, doc, undo, cache): "))
		filePath = logos.AskForInput("Target file (e.g., testes/script.go): ")

		if action != "undo" && action != "cache" {
			if _, err := os.Stat("prompt.md"); err == nil {
				if logos.AskForConfirmation("Found 'prompt.md'. Do you want to use it as instruction? (y/n): ") {
					data, _ := os.ReadFile("prompt.md")
					instruction = strings.TrimSpace(string(data))
				}
			}
			if instruction == "" {
				instruction = logos.AskForInput("Type your instruction: ")
			}
		}
	} else {
		action = strings.ToLower(flag.Arg(0))
		filePath = flag.Arg(1)

		if len(flag.Args()) >= 3 {
			instruction = flag.Arg(2)
		} else if action != "undo" && action != "cache" {
			if data, err := os.ReadFile("prompt.md"); err == nil {
				instruction = strings.TrimSpace(string(data))
				slog.Info("Instruction successfully loaded from prompt.md")
			} else {
				fmt.Println("\n[!] Error: No instruction provided and 'prompt.md' could not be found.")
				os.Exit(1)
			}
		}
	}

	// Mapeia os caminhos inteligentes e cria as pastas sob demanda
	paths := logos.ResolveMetaPaths(filePath)

	if action == "undo" {
		logos.Rollback(paths)
		return
	}

	contentStr, err := logos.ReadOrCreateFile(paths, action)
	if err != nil {
		slog.Error("Failed to read file", "error", err)
		os.Exit(1)
	}

	if action == "cache" {
		if err := logos.UpdateCache(paths, contentStr, cfg); err != nil {
			slog.Error("Failed to update structure cache", "error", err)
			os.Exit(1)
		}
		return
	}

	cacheContext, _ := logos.ReadFileIfExists(paths.Cache)

	slog.Info(fmt.Sprintf("Consulting %s [%s] for %s...", cfg.Model, action, paths.Source))

	newContent, aiSummary, tokens, err := logos.CallGroq(contentStr, action, instruction, cfg, cacheContext)
	if err != nil {
		slog.Error("AI prompt handling failed", "error", err)
		os.Exit(1)
	}
	newContent = logos.CleanMarkdown(newContent)

	if err := logos.ShowDiff(paths, newContent); err != nil {
		slog.Warn("Could not display diff visualizer", "error", err)
	}

	if *dryRun {
		fmt.Println("\nMode --dry-run active. No changes were applied.")
		fmt.Printf("Technical Summary:\n%s\n", aiSummary)
		return
	}

	if !logos.AskForConfirmation("Apply changes? (y/n): ") {
		fmt.Println("Cancelled.")
		return
	}

	if len(contentStr) > 0 {
		if err := os.WriteFile(paths.Bak, []byte(contentStr), 0600); err != nil {
			slog.Warn("Failed to create backup copy", "error", err)
		}
	}

	if err := os.WriteFile(paths.Source, []byte(newContent), 0644); err != nil {
		slog.Error("Failed to save changes to file", "error", err)
		os.Exit(1)
	}
	slog.Info("File saved successfully!")

	logos.AppendProgress(paths.Source, action, instruction, aiSummary, cfg.Model, tokens)

	slog.Info("Updating structural map cache...")
	logos.UpdateCache(paths, newContent, cfg)
}