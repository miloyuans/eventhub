package processor

import (
	"context"
	"eventhub/pkg/utils"
	"eventhub/sqlevent/internal/model"
	"log/slog"
	"time"

	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Processor struct {
	buffer     chan model.IngestRequest
	db         *mongo.Client
	cfg        *viper.Viper
	collection *mongo.Collection
}

func NewProcessor(db *mongo.Client, cfg *viper.Viper) *Processor {
	// 读取配置
	bufSize := cfg.GetInt("slow_sql.buffer_size")
	dbName := cfg.GetString("mongodb.db_name")
	
	return &Processor{
		buffer:     make(chan model.IngestRequest, bufSize),
		db:         db,
		cfg:        cfg,
		collection: db.Database(dbName).Collection("slow_sql_events"),
	}
}

func (p *Processor) Start() {
	workerCount := p.cfg.GetInt("slow_sql.workers")
	for i := 0; i < workerCount; i++ {
		go p.worker(i)
	}
	// 启动定期清理任务
	go p.cleaner()
}

// Push 将事件推入队列，非阻塞
func (p *Processor) Push(req model.IngestRequest) bool {
	select {
	case p.buffer <- req:
		return true
	default:
		slog.Warn("SlowSQL buffer full, dropping event", "env", req.Env)
		return false
	}
}

func (p *Processor) worker(id int) {
	for req := range p.buffer {
		// 生成去重ID: Content + Env + Account
		docID := utils.GenerateID(req.Content, req.Env, req.Account)
		
		eventTime := time.Now()
		if req.Time > 0 {
			eventTime = time.Unix(req.Time, 0)
		}

		// MongoDB Upsert 原子操作
		filter := bson.M{"_id": docID}
		update := bson.M{
			"$setOnInsert": bson.M{
				"fingerprint": utils.GenerateID(req.Content), // 仅内容的指纹
				"content":     req.Content,
				"env":         req.Env,
				"account":     req.Account,
				"status":      model.StatusPending, // 默认为待处理
				"created_at":  time.Now(),
				"remarks":     []model.Remark{},
			},
			"$inc": bson.M{"count": 1},
			"$set": bson.M{"last_seen": eventTime},
			"$push": bson.M{
				"timestamps": eventTime, // 追加时间点
			},
		}
		opts := options.Update().SetUpsert(true)

		_, err := p.collection.UpdateOne(context.TODO(), filter, update, opts)
		if err != nil {
			slog.Error("Failed to upsert event", "err", err, "id", docID)
		}
	}
}

// cleaner 清理超过保留周期的 timestamps 数组元素
func (p *Processor) cleaner() {
	ticker := time.NewTicker(4 * time.Hour)
	for range ticker.C {
		days := p.cfg.GetInt("slow_sql.retention_days")
		cutoff := time.Now().AddDate(0, 0, -days)

		slog.Info("Running cleaner", "cutoff", cutoff)

		// 移除 timestamps 数组中小于 cutoff 的元素
		_, err := p.collection.UpdateMany(context.TODO(), bson.M{}, bson.M{
			"$pull": bson.M{
				"timestamps": bson.M{"$lt": cutoff},
			},
		})
		if err != nil {
			slog.Error("Cleaner failed", "err", err)
		}
	}
}
