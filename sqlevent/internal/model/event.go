package model

import "time"

type EventStatus string

const (
	StatusPending    EventStatus = "pending"    // 待处理
	StatusProcessing EventStatus = "processing" // 处理中
	StatusResolved   EventStatus = "resolved"   // 已完成
	StatusIgnored    EventStatus = "ignored"    // 已忽略
)

// IngestRequest API 接收的原始数据
type IngestRequest struct {
	Type    string `json:"type" binding:"required"`
	Content string `json:"content" binding:"required"`
	Env     string `json:"env" binding:"required"`
	Account string `json:"account" binding:"required"`
	Time    int64  `json:"time"` // Unix Timestamp
}

// Remark 备注历史
type Remark struct {
	Time    time.Time `bson:"time" json:"time"`
	Content string    `bson:"content" json:"content"`
	User    string    `bson:"user" json:"user"`
}

// SlowSqlEvent 数据库存储结构
type SlowSqlEvent struct {
	ID          string      `bson:"_id" json:"id"`
	Fingerprint string      `bson:"fingerprint" json:"fingerprint"` // 内容指纹
	Content     string      `bson:"content" json:"content"`
	Env         string      `bson:"env" json:"env"`
	Account     string      `bson:"account" json:"account"`
	Count       int64       `bson:"count" json:"count"`
	Status      EventStatus `bson:"status" json:"status"`
	Remarks     []Remark    `bson:"remarks" json:"remarks"`
	
	// 保留最近7天的时间点，用于生成趋势图或判断频率
	Timestamps []time.Time `bson:"timestamps" json:"timestamps"`
	
	LastSeen  time.Time `bson:"last_seen" json:"last_seen"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}
