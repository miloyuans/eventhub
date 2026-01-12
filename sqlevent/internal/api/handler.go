package api

import (
	"context"
	"eventhub/sqlevent/internal/model"
	"eventhub/sqlevent/internal/processor"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// IngestHandler 接收 API 数据
func IngestHandler(p *processor.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req model.IngestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		// 异步处理，立即返回
		ok := p.Push(req)
		if !ok {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Buffer full"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "queued"})
	}
}

// UpdateStatusHandler 更新状态和备注
func UpdateStatusHandler(db *mongo.Client, cfg *viper.Viper) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ID     string            `json:"id"`
			Status model.EventStatus `json:"status"`
			Remark string            `json:"remark"`
			User   string            `json:"user"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		maxLen := cfg.GetInt("slow_sql.remark_max_len")
		if len(req.Remark) > maxLen {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Remark too long"})
			return
		}

		coll := db.Database(cfg.GetString("mongodb.db_name")).Collection("slow_sql_events")
		
		update := bson.M{"$set": bson.M{"status": req.Status}}
		if req.Remark != "" {
			update["$push"] = bson.M{"remarks": model.Remark{
				Time:    time.Now(),
				Content: req.Remark,
				User:    req.User,
			}}
		}

		_, err := coll.UpdateOne(context.TODO(), bson.M{"_id": req.ID}, update)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "updated"})
	}
}

// ListEventsHandler 简单的列表查询
func ListEventsHandler(db *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 实际项目中这里需要处理分页、筛选等
		c.JSON(http.StatusOK, gin.H{"message": "List API implemented here"})
	}
}
