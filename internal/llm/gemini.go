package llm

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/austinjan/aascribe/internal/apperr"
	"google.golang.org/genai"
)

type GeminiConfig struct {
	Model          string
	APIKey         string
	TimeoutSeconds int
	HTTPClient     *http.Client
}

type GeminiClient struct {
	model        string
	timeout      time.Duration
	genaiFactory func(context.Context, *genai.ClientConfig) (genaiClient, error)
	apiKey       string
	httpClient   *http.Client
}

type TextResponse struct {
	Provider     string
	Model        string
	Text         string
	FinishReason string
	Usage        Usage
}

type Usage struct {
	PromptTokenCount     int `json:"prompt_token_count"`
	CandidatesTokenCount int `json:"candidates_token_count"`
	TotalTokenCount      int `json:"total_token_count"`
}

type genaiClient interface {
	GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
}

type sdkClient struct {
	models *genai.Models
}

func (c *sdkClient) GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	return c.models.GenerateContent(ctx, model, contents, config)
}

func NewGeminiClient(cfg GeminiConfig) *GeminiClient {
	return &GeminiClient{
		model:      cfg.Model,
		apiKey:     cfg.APIKey,
		timeout:    time.Duration(cfg.TimeoutSeconds) * time.Second,
		httpClient: cfg.HTTPClient,
		genaiFactory: func(ctx context.Context, clientCfg *genai.ClientConfig) (genaiClient, error) {
			client, err := genai.NewClient(ctx, clientCfg)
			if err != nil {
				return nil, err
			}
			return &sdkClient{models: client.Models}, nil
		},
	}
}

func (c *GeminiClient) GenerateText(prompt string) (*TextResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	client, err := c.genaiFactory(ctx, &genai.ClientConfig{
		APIKey:     c.apiKey,
		Backend:    genai.BackendGeminiAPI,
		HTTPClient: c.httpClient,
	})
	if err != nil {
		return nil, apperr.IOError("Failed to create Gemini client: %s.", err)
	}

	response, err := client.GenerateContent(ctx, c.model, genai.Text(prompt), &genai.GenerateContentConfig{})
	if err != nil {
		return nil, apperr.IOError("Failed to call Gemini API: %s.", err)
	}
	if response == nil {
		return nil, apperr.IOError("Gemini returned no response.")
	}

	text := response.Text()
	if text == "" {
		return nil, apperr.IOError("Gemini returned an empty text response.")
	}

	finishReason := ""
	if len(response.Candidates) > 0 && response.Candidates[0] != nil {
		finishReason = fmt.Sprint(response.Candidates[0].FinishReason)
	}

	usage := Usage{}
	if response.UsageMetadata != nil {
		usage.PromptTokenCount = int(response.UsageMetadata.PromptTokenCount)
		usage.CandidatesTokenCount = int(response.UsageMetadata.CandidatesTokenCount)
		usage.TotalTokenCount = int(response.UsageMetadata.TotalTokenCount)
	}

	return &TextResponse{
		Provider:     "gemini",
		Model:        c.model,
		Text:         text,
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}
