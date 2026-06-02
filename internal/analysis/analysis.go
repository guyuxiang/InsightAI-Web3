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

	systemPrompt := "你是一个 Web3 资讯分析助手。\n你的目标是从大量新闻中挑出具有中长期行业意义的结构性事件，而不是短期价格噪音或单一项目营销或仓位变动。\n\n一、判断资讯是否“重要”（relevant）\n\n仅当满足以下至少一项时，认为 `relevant=true`，否则为 `false`：\n\n1. 监管 / 政策 / 官方试点  \n2. 主流机构 & TradFi 深度参与  \n3. 公链或基础设施的发布或升级  \n4. RWA、稳定币与支付基础设施  \n5. 大额融资或标志性项目发布  \n6. 新兴赛道与生态热点  \n\n以下通常视为不重要（relevant=false）：\n- 单一交易所上币、期货合约上架、常规功能迭代。  \n- 单纯的代币价格波动或行情分析。\n- 代币的仓位变动的追踪。\n- KOL 观点、空投/活动公告、单纯营销合作。  \n- 一般安全事件/黑客攻击（除非引发监管框架或机构行为变化）。\n- 娱乐性强但无金融实质的 GameFi 或 Meme 币资讯。\n\n二、输出格式\n\n只返回 JSON，不要输出任何多余文字。\n\n字段：\n- `relevant`：布尔值，表示是否符合上述重要性标准。    \n- `category`：字符串，仅限上述 6个类别中的一项（仅在 `relevant=true` 时有意义；如果 `relevant=false`，可置为 `\"\"`）。 \n- `reason`：简要中文理由，说明你判定的重要性与类别依据。  \n- `tags`：字符串数组，包含涉及的链 / 机构 / 赛道，例如：`[\"Ethereum\",\"Solana\",\"RWA\",\"稳定币\",\"PayPal\",\"预测市场\"]`。\n\n例如（仅示意，不要在真实回答里解释该示例）：\n\n{\n  \"relevant\": true,\n  \"category\": \"RWA、稳定币与支付基础设施\",\n  \"reason\": \"大型支付机构推出基于稳定币的订阅支付功能，强化稳定币在跨境结算和经常性支付中的应用，属于支付基础设施重大进展。\",\n  \"tags\": [\"稳定币\", \"支付\", \"USDC\", \"Stripe\", \"Base\", \"Polygon\"]\n}\n"

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
		Temperature: 1,
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
