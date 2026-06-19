package logos

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 90 * time.Second}

type FilePayload struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type AIResponse struct {
	Summary string        `json:"summary"`
	Files   []FilePayload `json:"files"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Provider interface {
	Generate(ctx context.Context, systemPrompt, userPrompt string, temperature float64) (string, int, error)
}

type UniversalAIClient struct {
	Cfg Config
}

func NewAIClient(cfg Config) Provider {
	return &UniversalAIClient{Cfg: cfg}
}

func (c *UniversalAIClient) Generate(ctx context.Context, systemPrompt, userPrompt string, temperature float64) (string, int, error) {
	maxRetries := 3
	baseDelay := 1 * time.Second

	var url string
	var body []byte
	var err error

	// 1. ROTEAMENTO NATIVO SEGURO (Chaves protegidas por HTTPS e encapsuladas em Headers)
	if c.Cfg.Provider == "gemini" {
		// URL limpa sem expor chaves via parâmetros expostos (?key=)
		url = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", c.Cfg.Model)

		combinedPrompt := fmt.Sprintf("%s\n\n[INSTRUÇÃO E CONTEXTO DO WORKSPACE]:\n%s", systemPrompt, userPrompt)
		geminiBody := map[string]any{
			"contents": []map[string]any{
				{
					"parts": []map[string]string{
						{"text": combinedPrompt},
					},
				},
			},
			"generationConfig": map[string]any{
				"temperature": temperature,
			},
		}
		body, err = json.Marshal(geminiBody)
	} else {
		url = "https://api.groq.com/openai/v1/chat/completions"
		groqBody := map[string]any{
			"model":       c.Cfg.Model,
			"temperature": temperature,
			"messages": []Message{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
			},
			"response_format": map[string]string{"type": "json_object"},
		}
		body, err = json.Marshal(groqBody)
	}

	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal body: %w", err)
	}

	// 2. DISPAROS DO CLIENTE COM TRATAMENTO DE RETENTATIVAS
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
		if err != nil {
			return "", 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		
		// Injeção de Segurança em Headers fechados de rede (Blinda as credenciais)
		if c.Cfg.Provider == "groq" {
			req.Header.Set("Authorization", "Bearer "+c.Cfg.APIKey)
		} else if c.Cfg.Provider == "gemini" {
			req.Header.Set("X-Goog-Api-Key", c.Cfg.APIKey)
		}

		slog.Debug("Requesting AI", "provider", c.Cfg.Provider, "attempt", attempt)
		resp, err := httpClient.Do(req)
		if err != nil {
			slog.Warn("Network error", "error", err, "attempt", attempt)
			if attempt == maxRetries {
				return "", 0, err
			}
			time.Sleep(baseDelay)
			baseDelay *= 2
			continue
		}

		if resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()

			if c.Cfg.Provider == "gemini" {
				var resGemini struct {
					Candidates []struct {
						Content struct {
							Parts []struct {
								Text string `json:"text"`
							} `json:"parts"`
						} `json:"content"`
					} `json:"candidates"`
					UsageMetadata struct {
						TotalTokenCount int `json:"totalTokenCount"`
					} `json:"usageMetadata"`
				}

				if err := json.NewDecoder(resp.Body).Decode(&resGemini); err != nil {
					return "", 0, fmt.Errorf("failed decoding Gemini JSON: %w", err)
				}
				if len(resGemini.Candidates) == 0 || len(resGemini.Candidates[0].Content.Parts) == 0 {
					return "", 0, errors.New("Gemini API returned an empty response")
				}

				return resGemini.Candidates[0].Content.Parts[0].Text, resGemini.UsageMetadata.TotalTokenCount, nil
			}

			var resGroq struct {
				Choices []struct {
					Message Message `json:"message"`
				} `json:"choices"`
				Usage struct {
					TotalTokens int `json:"total_tokens"`
				} `json:"usage"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&resGroq); err != nil {
				return "", 0, fmt.Errorf("failed decoding Groq JSON: %w", err)
			}
			if len(resGroq.Choices) == 0 {
				return "", 0, errors.New("Groq API response contains no choices")
			}

			return resGroq.Choices[0].Message.Content, resGroq.Usage.TotalTokens, nil
		}

		resp.Body.Close()
		if attempt == maxRetries {
			return "", 0, fmt.Errorf("API failed after %d attempts with status %d", maxRetries, resp.StatusCode)
		}
		time.Sleep(baseDelay)
		baseDelay *= 2
	}

	return "", 0, errors.New("unknown failure during API execution")
}

func ParseAIResponse(response string) (AIResponse, error) {
	var target AIResponse
	cleaned := strings.TrimSpace(response)
	
	if strings.HasPrefix(cleaned, "```json") {
		cleaned = strings.TrimPrefix(cleaned, "```json")
		cleaned = strings.TrimSuffix(cleaned, "```")
	} else if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(cleaned, "```")
	}
	cleaned = strings.TrimSpace(cleaned)

	err := json.Unmarshal([]byte(cleaned), &target)
	if err != nil {
		return target, fmt.Errorf("IA não retornou o JSON esperado: %w. Resposta: %s", err, response)
	}
	return target, nil
}

func UpdateCache(paths MetaPaths, files []FilePayload, client Provider, cfg Config) error {
	if len(files) == 0 {
		return nil
	}
	payloadBytes, _ := json.Marshal(files)
	currentHash := fmt.Sprintf("%x", sha256.Sum256(payloadBytes))

	oldHash, err := os.ReadFile(paths.Hash)
	if err == nil && string(oldHash) == currentHash {
		return nil
	}

	sysPrompt := "Você é um arquiteto de software. Retorne um mapa simples de assinaturas de funções e dependências."
	resText, _, err := client.Generate(context.Background(), sysPrompt, string(payloadBytes), 0.0)
	if err != nil {
		return err
	}

	_ = os.WriteFile(paths.Cache, []byte(resText), 0600)
	return os.WriteFile(paths.Hash, []byte(currentHash), 0600)
}