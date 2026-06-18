package logos

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 60 * time.Second}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func CallGroq(content, action, instruction string, cfg Config, cacheContext string) (string, string, int, error) {
	prompt, err := GetSystemPrompt(action, cacheContext, content, instruction)
	if err != nil {
		return "", "", 0, err
	}

	resText, tokens, err := doGroqRequest(prompt, 0.1, cfg)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to communicate with AI: %w", err)
	}

	code, summary := parseAIResponse(resText)
	return code, summary, tokens, nil
}

func UpdateCache(paths MetaPaths, content string, cfg Config) error {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	currentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	oldHash, err := os.ReadFile(paths.Hash)
	if err == nil && string(oldHash) == currentHash {
		slog.Debug("Cache up to date (content has not changed)")
		return nil
	}

	prompt := GetArchitectPrompt(content)

	slog.Debug("Generating new structural map via API...")
	resText, _, err := doGroqRequest(prompt, 0.0, cfg)
	if err != nil {
		return fmt.Errorf("error in UpdateCache: %w", err)
	}

	if err := os.WriteFile(paths.Cache, []byte(resText), 0600); err != nil {
		return err
	}
	return os.WriteFile(paths.Hash, []byte(currentHash), 0600)
}

func doGroqRequest(prompt string, temperature float64, cfg Config) (string, int, error) {
	maxRetries := 3
	baseDelay := 1 * time.Second

	body, err := json.Marshal(map[string]any{
		"model":       cfg.Model,
		"temperature": temperature,
		"messages":    []Message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal body: %w", err)
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(body))
		if err != nil {
			return "", 0, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

		slog.Debug("Sending request to Groq", "attempt", attempt)
		resp, err := httpClient.Do(req)

		if err != nil {
			slog.Warn("Network error consulting Groq", "error", err, "attempt", attempt)
			if attempt == maxRetries {
				return "", 0, fmt.Errorf("failed after %d attempts: %w", maxRetries, err)
			}
			time.Sleep(baseDelay)
			baseDelay *= 2
			continue
		}

		if resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var resData struct {
				Choices []struct {
					Message Message `json:"message"`
				} `json:"choices"`
				Usage struct {
					TotalTokens int `json:"total_tokens"`
				} `json:"usage"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&resData); err != nil {
				return "", 0, fmt.Errorf("failed decoding JSON response: %w", err)
			}
			if len(resData.Choices) == 0 {
				return "", 0, errors.New("API response contains no choices")
			}

			return resData.Choices[0].Message.Content, resData.Usage.TotalTokens, nil
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		slog.Warn("API returned an error", "status", resp.StatusCode, "body", string(bodyBytes))

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			if attempt == maxRetries {
				return "", 0, fmt.Errorf("API failed after %d attempts with status %d", maxRetries, resp.StatusCode)
			}
			time.Sleep(baseDelay)
			baseDelay *= 2
		} else {
			return "", 0, fmt.Errorf("API returned non-recoverable status %d: %s", resp.StatusCode, string(bodyBytes))
		}
	}

	return "", 0, errors.New("unknown failure during API retries")
}

func parseAIResponse(response string) (string, string) {
	var summary, code string
	if start := strings.Index(response, "<progress>"); start != -1 {
		if end := strings.Index(response, "</progress>"); end != -1 {
			summary = strings.TrimSpace(response[start+len("<progress>") : end])
		}
	}
	if start := strings.Index(response, "<code>"); start != -1 {
		if end := strings.LastIndex(response, "</code>"); end != -1 {
			code = strings.TrimSpace(response[start+len("<code>") : end])
		}
	}
	if code == "" {
		code = strings.TrimSpace(response)
	}
	if summary == "" {
		summary = "Technical modification applied according to instruction."
	}

	code = html.UnescapeString(code)
	return code, summary
}