package logos

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Definição de estilos ANSI minimalistas
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	FgGray  = "\033[38;5;244m" // Cinza sutil para divisórias contínuas
)

// ShowWorkspaceDiff exibe as alterações de forma limpa sem poluição visual
func ShowWorkspaceDiff(paths MetaPaths, proposedFiles []FilePayload) error {
	fmt.Println(FgGray + "────────────────────────────────────────────────────────────────────────" + Reset)
	for _, file := range proposedFiles {
		origPathForDiff := file.Path
		if _, err := os.Stat(file.Path); os.IsNotExist(err) {
			origPathForDiff = os.DevNull
		}

		fmt.Printf("%sArquivo: %s%s\n", Bold, file.Path, Reset)
		
		tmpNew, err := os.CreateTemp("", "logos_diff_new")
		if err != nil {
			return err
		}
		defer os.Remove(tmpNew.Name())
		_, _ = tmpNew.WriteString(file.Content)
		_ = tmpNew.Close()

		cmd := exec.Command("diff", "-u", "--color=always", origPathForDiff, tmpNew.Name())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		fmt.Println(FgGray + "────────────────────────────────────────────────────────────────────────" + Reset)
	}
	return nil
}

// AskForInput captura dados do terminal de forma direta
func AskForInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// AskForConfirmation exibe um menu baseado na referência clássica e limpa enviada
func AskForConfirmation(prompt string) bool {
	fmt.Println(Bold + "============== Logos System Action ==============" + Reset)
	fmt.Printf("1. %s\n", prompt)
	fmt.Println("Q. Cancelar operação")
	fmt.Println(FgGray + "=================================================" + Reset)
	
	res := AskForInput("Please make a selection (1/Q): ")
	res = strings.ToLower(res)
	
	// Aceita "1", "s", "sim", "y" ou enter direto como confirmação padrão rápida
	return res == "1" || res == "s" || res == "sim" || res == "y" || res == ""
}

// PrintUsage exibe o help de comandos de forma limpa e tabular
func PrintUsage() {
	fmt.Println(Bold + "\nLOGOS CLI" + Reset)
	fmt.Println(FgGray + "Uso: logos [modificadores] <ação> <arquivos...> [instrução]\n" + Reset)
	
	fmt.Println(Bold + "Ações:" + Reset)
	fmt.Printf("  %-12s %s\n", "feat", "Cria ou insere uma nova lógica no escopo.")
	fmt.Printf("  %-12s %s\n", "fix", "Localiza falhas e corrige estritamente as linhas defeituosas.")
	fmt.Printf("  %-12s %s\n", "refactor", "Aplica Clean Code sem alterar o comportamento atual.")
	fmt.Printf("  %-12s %s\n", "doc", "Insere documentação técnica útil nas assinaturas.")
	fmt.Printf("  %-12s %s\n", "cache", "Força a recomputação estrutural do arquivo alvo.")
	fmt.Printf("  %-12s %s\n", "undo", "Executa rollbacks restaurando o estado anterior.")
	
	fmt.Println(Bold + "\nModificadores:" + Reset)
	fmt.Printf("  %-12s %s\n", "-gemini", "Chaveia o fluxo para o modelo gemini-2.5-flash.")
	fmt.Printf("  %-12s %s\n", "-groq", "Chaveia o fluxo para o modelo llama-3.3-70b-versatile.")
	fmt.Printf("  %-12s %s\n", "--dry-run", "Apenas renderiza o diff na tela sem gravar em disco.")
	fmt.Println()
}