package main

import (
	"fmt"
	"strings"
	"time"

	"go-game-leaderboard/internal/service"
	"go-game-leaderboard/internal/repository"
)

func main() {
	fmt.Println("===== 游戏排行榜系统测试 =====\n")

	// 测试本地内存版
	testLocalLeaderboard()

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// 测试 Redis 版（需要启动 Redis 服务）
	// 取消下面这行的注释以测试 Redis
	// testRedisLeaderboard()
}

func testLocalLeaderboard() {
	fmt.Println("【本地内存版测试】")
	lb := service.NewLeaderboard()

	// 模拟玩家数据
	players := []struct {
		id        string
		score     float64
		timestamp int64
	}{
		{"player1", 100, 1000},
		{"player2", 150, 1100},
		{"player3", 120, 1200},
		{"player4", 150, 1050}, // 与 player2 同分但时间更早，应排前面
		{"player5", 90, 1300},
		{"player6", 95, 1400},
	}

	// 更新分数
	for _, p := range players {
		lb.UpdateScore(p.id, p.score, p.timestamp)
	}

	// 测试1：获取前3名
	fmt.Println("\n【测试1：获取前3名】")
	top3 := lb.GetTopN(3)
	for _, info := range top3 {
		fmt.Printf("排名 %d: %s - 分数: %.0f\n", info.Rank, info.PlayerId, info.Score)
	}

	// 测试2：查询玩家排名
	fmt.Println("\n【测试2：查询玩家排名】")
	rank := lb.GetPlayerRank("player3")
	if rank != nil {
		fmt.Printf("%s 排名: %d, 分数: %.0f\n", rank.PlayerId, rank.Rank, rank.Score)
	}

	// 测试3：查询周边排名
	fmt.Println("\n【测试3：查询 player3 周边排名】")
	rangeInfo := lb.GetPlayerRankRange("player3", 4)
	for _, info := range rangeInfo {
		fmt.Printf("排名 %d: %s - 分数: %.0f\n", info.Rank, info.PlayerId, info.Score)
	}

	// 测试4：密集排名
	fmt.Println("\n【测试4：密集排名】")
	topNDense := lb.GetTopNDense(10)
	for _, info := range topNDense {
		fmt.Printf("排名 %d: %s - 分数: %.0f\n", info.Rank, info.PlayerId, info.Score)
	}

	// 测试5：更新玩家分数
	fmt.Println("\n【测试5：更新 player1 分数 +60】")
	lb.UpdateScore("player1", 60, time.Now().Unix())
	rank = lb.GetPlayerRank("player1")
	if rank != nil {
		fmt.Printf("%s 新排名: %d, 新分数: %.0f\n", rank.PlayerId, rank.Rank, rank.Score)
	}
}

func testRedisLeaderboard() {
	fmt.Println("【Redis 版测试】")
	rl := repository.NewRedisLeaderboard("localhost:6379")
	defer rl.Close()

	// 模拟玩家数据
	players := []struct {
		id        string
		score     float64
		timestamp int64
	}{
		{"player1", 100, 1000},
		{"player2", 150, 1100},
		{"player3", 120, 1200},
		{"player4", 150, 1050},
		{"player5", 90, 1300},
		{"player6", 95, 1400},
	}

	// 更新分数
	for _, p := range players {
		if err := rl.UpdateScore(p.id, p.score, p.timestamp); err != nil {
			fmt.Printf("更新失败: %v\n", err)
		}
	}

	// 测试：获取前3名
	fmt.Println("\n【获取前3名】")
	top3, err := rl.GetTopN(3)
	if err != nil {
		fmt.Printf("查询失败: %v\n", err)
		return
	}
	for _, info := range top3 {
		fmt.Printf("排名 %d: %s - 分数: %.0f\n", info.Rank, info.PlayerId, info.Score)
	}

	// 测试：密集排名
	fmt.Println("\n【密集排名】")
	rank, err := rl.GetPlayerRankDense("player4")
	if err != nil {
		fmt.Printf("查询失败: %v\n", err)
		return
	}
	fmt.Printf("%s 密集排名: %d, 分数: %.0f\n", rank.PlayerId, rank.Rank, rank.Score)
}
