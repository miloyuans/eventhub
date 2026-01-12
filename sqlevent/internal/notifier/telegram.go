package notifier

import (
	"bytes"
	"context"
	"eventhub/sqlevent/internal/model"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Notifier struct {
	db  *mongo.Client
	cfg *viper.Viper
}

func NewNotifier(db *mongo.Client, cfg *viper.Viper) *Notifier {
	return &Notifier{db: db, cfg: cfg}
}

func (n *Notifier) Start() {
	if !n.cfg.GetBool("slow_sql.telegram.enabled") {
		return
	}
	
	interval := n.cfg.GetInt("slow_sql.telegram.interval_minutes")
	ticker := time.NewTicker(time.Duration(interval) * time.Minute)
	
	go func() {
		for range ticker.C {
			n.generateAndSend()
		}
	}()
}

func (n *Notifier) generateAndSend() {
	dbName := n.cfg.GetString("mongodb.db_name")
	coll := n.db.Database(dbName).Collection("slow_sql_events")

	// 查找最近一个周期内活跃的事件
	minutes := n.cfg.GetInt("slow_sql.telegram.interval_minutes")
	since := time.Now().Add(-time.Duration(minutes) * time.Minute)

	filter := bson.M{"last_seen": bson.M{"$gte": since}}
	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		slog.Error("Notifier DB error", "err", err)
		return
	}
	
	var events []model.SlowSqlEvent
	if err := cursor.All(context.TODO(), &events); err != nil {
		return
	}

	if len(events) == 0 {
		return
	}

	n.sendReport(events)
}

func (n *Notifier) sendReport(events []model.SlowSqlEvent) {
	// 1. 生成统计信息
	stats := map[string]int{"Pending": 0, "Processing": 0, "Resolved": 0, "Ignored": 0}
	envCount := map[string]int{}
	
	for _, e := range events {
		stats[string(e.Status)]++ // 这里假设 Status 字段值首字母大写或自行转换
		envCount[e.Env]++
	}

	// 2. 构建概览消息
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("🐢 **EventHub 慢SQL 周期报告**\n"))
	buf.WriteString(fmt.Sprintf("🕒 时间: %s\n", time.Now().Format("2006-01-02 15:04")))
	buf.WriteString(fmt.Sprintf("📊 本周期触发: %d 类事件\n", len(events)))
	buf.WriteString("------------------\n")
	buf.WriteString(fmt.Sprintf("🔴 待处理: %d | 🟡 处理中: %d\n", stats["pending"], stats["processing"]))
	buf.WriteString(fmt.Sprintf("🟢 已完成: %d | ⚪ 已忽略: %d\n", stats["resolved"], stats["ignored"]))
	buf.WriteString("\n**环境分布**:\n")
	for env, cnt := range envCount {
		buf.WriteString(fmt.Sprintf("- %s: %d\n", env, cnt))
	}

	summaryText := buf.String()

	// 3. 构建详情 (Markdown)
	var detailBuf bytes.Buffer
	detailBuf.WriteString("# Slow SQL Detail Report\n\n")
	for _, e := range events {
		detailBuf.WriteString(fmt.Sprintf("## [%s] %s (Count: %d)\n", e.Env, e.Status, e.Count))
		detailBuf.WriteString(fmt.Sprintf("- **Account**: %s\n", e.Account))
		detailBuf.WriteString(fmt.Sprintf("- **Last Seen**: %s\n", e.LastSeen.Format(time.RFC3339)))
		detailBuf.WriteString(fmt.Sprintf("```sql\n%s\n```\n\n", e.Content))
	}
	
	detailBytes := detailBuf.Bytes()
	threshold := n.cfg.GetInt("slow_sql.telegram.send_file_threshold")

	// 4. 发送逻辑
	token := n.cfg.GetString("slow_sql.telegram.token")
	chatIDs := n.cfg.GetIntSlice("slow_sql.telegram.chat_ids")

	for _, chatID := range chatIDs {
		// 如果详情过大，发送 Summary 文本 + Detail 文件
		if len(detailBytes) > threshold {
			n.sendTelegramFile(token, chatID, summaryText, "report.md", detailBytes)
		} else {
			// 否则直接合并发送
			fullMsg := summaryText + "\n------------------\n" + detailBuf.String()
			n.sendTelegramMessage(token, chatID, fullMsg)
		}
	}
}

func (n *Notifier) sendTelegramMessage(token string, chatID int, text string) {
	// 实现简单的 JSON POST 请求到 https://api.telegram.org/bot<token>/sendMessage
	// 注意处理 Telegram 消息长度限制 (4096 字符)
	slog.Info("Sending Telegram Message", "chatID", chatID)
}

func (n *Notifier) sendTelegramFile(token string, chatID int, caption, filename string, fileData []byte) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", token)
	
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	writer.WriteField("chat_id", fmt.Sprintf("%d", chatID))
	writer.WriteField("caption", caption)
	
	part, _ := writer.CreateFormFile("document", filename)
	part.Write(fileData)
	writer.Close()

	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to send file", "err", err)
		return
	}
	defer resp.Body.Close()
}
