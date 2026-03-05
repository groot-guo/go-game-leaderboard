package repository

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/go-redis/redis/v8"

	"go-game-leaderboard/internal/model"
)

const (
	MaxTimestamp = 4294967295  // 时间戳最大值
	ScoreOffset  = 10000000000 // 分数偏移量
)

// RedisLeaderboard Redis 实现的排行榜
type RedisLeaderboard struct {
	client         *redis.Client
	ctx            context.Context
	leaderboardKey string
	scoresSetKey   string
	playerInfoKey  string
}

// NewRedisLeaderboard 创建 Redis 排行榜
func NewRedisLeaderboard(addr string) *RedisLeaderboard {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})
	return &RedisLeaderboard{
		client:         rdb,
		ctx:            context.Background(),
		leaderboardKey: "leaderboard:global",
		scoresSetKey:   "leaderboard:scores_set",
		playerInfoKey:  "leaderboard:player_info",
	}
}

// calculateCompositeScore 计算复合分数
func (rl *RedisLeaderboard) calculateCompositeScore(score float64, timestamp int64) float64 {
	tsPart := float64(MaxTimestamp - uint32(timestamp))
	return score*ScoreOffset + tsPart
}

// extractActualScore 从复合分数还原真实分数
func (rl *RedisLeaderboard) extractActualScore(compositeScore float64) float64 {
	return math.Floor(compositeScore / ScoreOffset)
}

// UpdateScore 更新玩家分数
func (rl *RedisLeaderboard) UpdateScore(playerId string, incrScore float64, timestamp int64) error {
	// 获取当前分数
	currentScore, err := rl.client.ZScore(rl.ctx, rl.leaderboardKey, playerId).Result()
	if err == redis.Nil {
		currentScore = 0
	} else if err != nil {
		return err
	}

	// 计算新分数
	actualScore := rl.extractActualScore(currentScore)
	newScore := actualScore + incrScore

	// 计算复合分数
	compositeScore := rl.calculateCompositeScore(newScore, timestamp)

	// 使用事务保证原子性
	pipe := rl.client.Pipeline()

	// 更新排行榜
	pipe.ZAdd(rl.ctx, rl.leaderboardKey, &redis.Z{
		Score:  compositeScore,
		Member: playerId,
	})

	// 更新分数集合（用于密集排名）
	pipe.ZAdd(rl.ctx, rl.scoresSetKey, &redis.Z{
		Score:  newScore,
		Member: newScore,
	})

	// 更新玩家信息
	pipe.HSet(rl.ctx, rl.playerInfoKey, playerId, fmt.Sprintf("%.0f:%d", newScore, timestamp))

	_, err = pipe.Exec(rl.ctx)
	return err
}

// GetPlayerRank 获取玩家排名
func (rl *RedisLeaderboard) GetPlayerRank(playerId string) (*model.RankInfo, error) {
	rank, err := rl.client.ZRevRank(rl.ctx, rl.leaderboardKey, playerId).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("player not found")
	}
	if err != nil {
		return nil, err
	}

	compositeScore, err := rl.client.ZScore(rl.ctx, rl.leaderboardKey, playerId).Result()
	if err != nil {
		return nil, err
	}

	actualScore := rl.extractActualScore(compositeScore)

	return &model.RankInfo{
		PlayerId: playerId,
		Score:    actualScore,
		Rank:     rank + 1,
	}, nil
}

// GetTopN 获取前N名
func (rl *RedisLeaderboard) GetTopN(n int64) ([]*model.RankInfo, error) {
	result, err := rl.client.ZRevRangeWithScores(rl.ctx, rl.leaderboardKey, 0, n-1).Result()
	if err != nil {
		return nil, err
	}

	list := make([]*model.RankInfo, 0, len(result))
	for i, z := range result {
		list = append(list, &model.RankInfo{
			PlayerId: z.Member.(string),
			Score:    rl.extractActualScore(z.Score),
			Rank:     int64(i + 1),
		})
	}
	return list, nil
}

// GetPlayerRankRange 获取周边排名
func (rl *RedisLeaderboard) GetPlayerRankRange(playerId string, rangeN int64) ([]*model.RankInfo, error) {
	rank, err := rl.client.ZRevRank(rl.ctx, rl.leaderboardKey, playerId).Result()
	if err != nil {
		return nil, err
	}

	start := rank - rangeN/2
	end := rank + rangeN/2

	if start < 0 {
		start = 0
	}

	result, err := rl.client.ZRevRangeWithScores(rl.ctx, rl.leaderboardKey, start, end).Result()
	if err != nil {
		return nil, err
	}

	list := make([]*model.RankInfo, 0, len(result))
	for i, z := range result {
		actualRank := start + int64(i) + 1
		list = append(list, &model.RankInfo{
			PlayerId: z.Member.(string),
			Score:    rl.extractActualScore(z.Score),
			Rank:     actualRank,
		})
	}
	return list, nil
}

// GetPlayerRankDense 获取玩家密集排名
func (rl *RedisLeaderboard) GetPlayerRankDense(playerId string) (*model.RankInfo, error) {
	compositeScore, err := rl.client.ZScore(rl.ctx, rl.leaderboardKey, playerId).Result()
	if err != nil {
		return nil, err
	}
	actualScore := rl.extractActualScore(compositeScore)

	rank, err := rl.client.ZRevRank(rl.ctx, rl.scoresSetKey, playerId).Result()
	if err != nil {
		return nil, err
	}

	return &model.RankInfo{
		PlayerId: playerId,
		Score:    actualScore,
		Rank:     rank + 1,
	}, nil
}

// GetTopNDense 获取密集排名的前N名
func (rl *RedisLeaderboard) GetTopNDense(n int64) ([]*model.RankInfo, error) {
	scores, err := rl.client.ZRevRange(rl.ctx, rl.scoresSetKey, 0, n-1).Result()
	if err != nil {
		return nil, err
	}

	result := make([]*model.RankInfo, 0)
	for rank, scoreStr := range scores {
		members, err := rl.client.ZRangeByScore(rl.ctx, rl.leaderboardKey, &redis.ZRangeBy{
			Min:   scoreStr,
			Max:   scoreStr,
			Count: 1,
		}).Result()
		if err != nil || len(members) == 0 {
			continue
		}

		scoreFloat, _ := strconv.ParseFloat(scoreStr, 64)

		result = append(result, &model.RankInfo{
			PlayerId: members[0],
			Score:    scoreFloat,
			Rank:     int64(rank + 1),
		})
	}

	return result, nil
}

// Close 关闭连接
func (rl *RedisLeaderboard) Close() error {
	return rl.client.Close()
}
