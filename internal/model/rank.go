package model

// RankInfo 排名信息
type RankInfo struct {
	PlayerId  string  `json:"playerId"`
	Score     float64 `json:"score"`
	Rank      int64   `json:"rank"`
	Timestamp int64   `json:"timestamp"`
}

// PlayerData 玩家数据
type PlayerData struct {
	PlayerId  string
	Score     float64
	Timestamp int64
}

// UpdateRequest 更新请求
type UpdateRequest struct {
	PlayerId  string
	IncrScore float64
	Timestamp int64
}
