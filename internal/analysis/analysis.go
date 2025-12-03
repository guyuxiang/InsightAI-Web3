package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// Result captures the AI judgement about a news item.
type Result struct {
	Relevant bool     `json:"relevant"`
	Category string   `json:"category"`
	Reason   string   `json:"reason"`
	Tags     []string `json:"tags"`
}

// Analyzer abstracts AI powered classification.
type Analyzer interface {
	Evaluate(ctx context.Context, req ItemContext) (Result, error)
	Ready() bool
}

// ItemContext contains the fields passed to the model.
type ItemContext struct {
	Title       string
	Link        string
	PublishedAt time.Time
	Summary     string
}

var errDisabled = errors.New("openai client disabled: missing OPENAI_API_KEY")

// Client implements Analyzer using the OpenAI chat completion API.
type Client struct {
	client    *openai.Client
	model     string
	logger    *log.Logger
	activated bool
}

// NewClient builds a new Analyzer. If apiKey is empty, calls will be no-op with errors.
func NewClient(apiKey, model, baseURL string, logger *log.Logger) *Client {
	var cli *openai.Client
	activated := apiKey != ""
	if activated {
		cfg := openai.DefaultConfig(apiKey)
		if baseURL != "" {
			cfg.BaseURL = baseURL
		}
		cli = openai.NewClientWithConfig(cfg)
	}
	return &Client{
		client:    cli,
		model:     model,
		logger:    logger,
		activated: activated,
	}
}

// Ready indicates whether the analyzer is usable.
func (c *Client) Ready() bool {
	return c.activated && c.client != nil
}

// Evaluate asks the model to categorize the news item and decide whether it matches our criteria.
func (c *Client) Evaluate(ctx context.Context, item ItemContext) (Result, error) {
	if !c.Ready() {
		return Result{}, errDisabled
	}

	systemPrompt := "你是一个Web3资讯分析助手。判断资讯是否属于以下类型：" +
		"1)重要政策或监管动向；2)行业重点项目或重大落地进展；3)创新新兴赛道或生态热点；4)RWA或支付赛道相关；5)投融资与机构动作；6)安全事件。" +
		"只返回JSON，字段：relevant(bool)，category(上述类别之一)，reason(简要中文理由)，tags(字符串数组，包含涉及的链/机构/赛道)。"
	userPrompt := fmt.Sprintf("标题: %s\n链接: %s\n发布时间: %s\n摘要: %s\n请输出JSON。",
		item.Title,
		item.Link,
		item.PublishedAt.Format(time.RFC3339),
		trimText(item.Summary, 800),
	)

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return Result{}, err
	}
	if len(resp.Choices) == 0 {
		return Result{}, errors.New("no choices returned by OpenAI")
	}

	content := cleanupResponse(resp.Choices[0].Message.Content)
	var out Result
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		c.logger.Printf("failed to parse OpenAI response, content=%q, err=%v", content, err)
		return Result{}, fmt.Errorf("parse openai response: %w", err)
	}

	return out, nil
}

func trimText(s string, max int) string {
	if len([]rune(s)) <= max {
		return s
	}
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}

// cleanupResponse removes code fences and normalizes minor model deviations.
func cleanupResponse(s string) string {
	c := strings.TrimSpace(s)
	if strings.HasPrefix(c, "```") {
		if idx := strings.Index(c, "\n"); idx != -1 {
			c = c[idx+1:]
		}
		c = strings.TrimPrefix(c, "```json")
		c = strings.TrimPrefix(c, "```")
		c = strings.TrimSuffix(c, "```")
		c = strings.TrimSpace(c)
	}
	// If model returns null for category, coerce to empty string to keep JSON valid for struct.
	c = strings.ReplaceAll(c, "\"category\": null", "\"category\": \"\"")
	return c
}
