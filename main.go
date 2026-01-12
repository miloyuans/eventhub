package main

import (
	"context"
	"eventhub/sqlevent"
	"flag"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	configFile := flag.String("config", "config/config.yaml", "Path to config file")
	flag.Parse()

	// 1. 加载配置
	viper.SetConfigFile(*configFile)
	if err := viper.ReadInConfig(); err != nil {
		panic("Failed to read config: " + err.Error())
	}

	// 2. 初始化数据库
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	mongoURI := viper.GetString("mongodb.uri")
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		panic("Failed to connect to Mongo: " + err.Error())
	}
	defer func() {
		if err = client.Disconnect(context.Background()); err != nil {
			slog.Error("Mongo disconnect error", "err", err)
		}
	}()

	// 3. 初始化 Gin
	r := gin.Default()

	// 4. 初始化模块
	// --- 慢SQL模块 ---
	sqlModule := sqlevent.Initialize(client, viper.GetViper())
	sqlModule.RegisterRoutes(r, client, viper.GetViper())

	// --- 后续扩展 H5 模块 ---
	// h5Module := h5event.Initialize(...)
	// h5Module.RegisterRoutes(...)

	// 5. 启动服务
	port := viper.GetString("server.port")
	slog.Info("EventHub Server Starting", "port", port)
	if err := r.Run(port); err != nil {
		panic(err)
	}
}
