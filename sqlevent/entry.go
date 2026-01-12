package sqlevent

import (
	//"context"
	"eventhub/sqlevent/internal/api"
	"eventhub/sqlevent/internal/notifier"
	"eventhub/sqlevent/internal/processor"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/mongo"
)

// Module 封装模块的生命周期
type Module struct {
	proc *processor.Processor
	not  *notifier.Notifier
}

// Initialize 初始化模块
func Initialize(db *mongo.Client, cfg *viper.Viper) *Module {
	slog.Info("Initializing Slow SQL Module...")

	// 1. 初始化处理器 (Worker Pool)
	proc := processor.NewProcessor(db, cfg)
	proc.Start()

	// 2. 初始化通知器 (Telegram)
	not := notifier.NewNotifier(db, cfg)
	not.Start()

	return &Module{
		proc: proc,
		not:  not,
	}
}

// RegisterRoutes 注册该模块的路由到 Gin 引擎
func (m *Module) RegisterRoutes(r *gin.Engine, db *mongo.Client, cfg *viper.Viper) {
	g := r.Group("/api/v1/sqlevent")
	{
		// 接收事件 (高并发)
		g.POST("/ingest", api.IngestHandler(m.proc))
		
		// 管理接口
		g.POST("/status", api.UpdateStatusHandler(db, cfg))
		g.GET("/list", api.ListEventsHandler(db))
	}
}
