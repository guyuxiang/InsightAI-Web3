# syntax=docker/dockerfile:1

# Build stage
FROM ghcr.io/guyuxiang/golang:1.23.4-alpine3.20 AS builder
WORKDIR /app
ENV CGO_ENABLED 0
ENV GOOS linux
ENV GO111MODULE=on
ENV GOARCH=amd64
ENV GOPROXY=https://mirrors.aliyun.com/goproxy/,direct
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o aiweb3news ./cmd/aiweb3news

# Runtime stage
FROM ghcr.io/guyuxiang/golang:1.23.4-alpine3.20
WORKDIR /app
COPY --from=builder /app/aiweb3news /app/aiweb3news

ENV FEED_URL=https://www.techflowpost.com/rss.aspx
ENV POLL_INTERVAL_MINUTES=15
ENV BIND_ADDR=:8082
ENV MAX_ITEMS=50

ENV OPENAI_API_KEY=sk-HT76re1vabQAuc4v17F2D13a61A4408a8930C143B1D38bA9
ENV OPENAI_MODEL=gpt-4o
ENV OPENAI_BASE_URL=https://aigateway.hrlyit.com/v1

ENV DB_HOST=mysql01.dev.lls.com
ENV DB_PORT=4120
ENV DB_USER=root
ENV DB_PASSWORD=123456
ENV DB_NAME=aiweb3news


EXPOSE 8082

ENTRYPOINT ["/app/aiweb3news"]
