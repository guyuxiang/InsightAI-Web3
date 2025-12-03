# AI Web3 News Filter

Golang 服务轮询 TechFlow 的 RSS (`https://www.techflowpost.com/rss.aspx`)，调用 ChatGPT 分析资讯，筛选以下类型：

- 重要政策/监管动向
- 行业重点项目、重大落地
- 创新新兴赛道或生态热点
- RWA 与支付赛道
- 投融资与机构动作
- 安全事件

符合条件的资讯存储在 MySQL 中，并通过 `/items` 接口返回。

## 快速开始

```bash
# 使用提供的网关与模型
export OPENAI_API_KEY=sk-HT76re1vabQAuc4v17F2D13a61A4408a8930C143B1D38bA9
export OPENAI_BASE_URL=https://aigateway.hrlyit.com/v1
export OPENAI_MODEL=gpt-4o
# MySQL 信息（可覆盖）
export DB_HOST=mysql01.dev.lls.com
export DB_PORT=4120
export DB_USER=root
export DB_PASSWORD=123456
export DB_NAME=aiweb3news
go run ./cmd/aiweb3news
# 服务默认监听 :8082
```

可选环境变量：

- `FEED_URL`：RSS 地址，默认 `https://www.techflowpost.com/rss.aspx`
- `POLL_INTERVAL_MINUTES`：轮询间隔分钟数，默认 15
- `BIND_ADDR`：HTTP 监听地址，默认 `:8082`
- `OPENAI_MODEL`：OpenAI 模型，默认 `gpt-4o`
- `OPENAI_BASE_URL`：OpenAI 网关地址，默认 `https://aigateway.hrlyit.com/v1`
- `MAX_ITEMS`：内存中保存的结果条数上限，默认 50
- `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME`：MySQL 连接信息（默认使用提供的实例）

## HTTP 接口

- `GET /healthz`：健康检查
- `GET /items`：返回筛选结果，字段包含标题、链接、发布时间、分类、理由及标签

## 工作流

1. 定时拉取 RSS
2. 将每条资讯摘要与元信息发送到 ChatGPT：
   - 判断是否属于指定类型
   - 返回分类、理由、标签
3. 将所有分析结果存入 MySQL（表：`news_analysis`），接口 `/items` 读取数据库返回“相关”资讯

## 开发提示

- 需要网络访问 RSS 和 OpenAI；未设置 `OPENAI_API_KEY` 时分析会失败
- 可通过 `curl http://localhost:8082/items` 查看最新过滤结果
