package service

import (
	"math/rand"
	"sync"
	"time"

	"go-game-leaderboard/internal/model"
)

// LeaderboardNode 跳表节点
type LeaderboardNode struct {
	data     PlayerData
	backward *LeaderboardNode
	levels   []*LeaderboardLevel
}

// LeaderboardLevel 跳表层
type LeaderboardLevel struct {
	forward *LeaderboardNode
	span    int
}

// PlayerData 玩家数据
type PlayerData struct {
	PlayerId  string
	Score     float64
	Timestamp int64
}

// Leaderboard 排行榜（本地内存版）
type Leaderboard struct {
	mu          sync.RWMutex
	header      *LeaderboardNode
	level       int
	length      int
	playerMap   map[string]*LeaderboardNode
	maxLevel    int
	probability float64
}

// NewLeaderboard 创建新排行榜
func NewLeaderboard() *Leaderboard {
	lb := &Leaderboard{
		level:       1,
		maxLevel:    16,
		probability: 0.25,
		playerMap:   make(map[string]*LeaderboardNode),
	}

	// 创建头节点
	lb.header = &LeaderboardNode{
		levels: make([]*LeaderboardLevel, lb.maxLevel),
	}
	for i := 0; i < lb.maxLevel; i++ {
		lb.header.levels[i] = &LeaderboardLevel{
			span: 0,
		}
	}

	return lb
}

// compareNodes 比较两个节点的排序顺序
func (lb *Leaderboard) compareNodes(a, b PlayerData) bool {
	if a.Score != b.Score {
		return a.Score > b.Score // 分数降序
	}
	return a.Timestamp < b.Timestamp // 时间戳升序（先得分的排前面）
}

// randomLevel 随机生成层级
func (lb *Leaderboard) randomLevel() int {
	level := 1
	for level < lb.maxLevel && rand.Float64() < lb.probability {
		level++
	}
	return level
}

// UpdateScore 更新玩家分数
func (lb *Leaderboard) UpdateScore(playerId string, incrScore float64, timestamp int64) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// 检查玩家是否存在
	if node, exists := lb.playerMap[playerId]; exists {
		// 删除旧节点
		lb.deleteNode(node)
		// 更新分数
		node.data.Score += incrScore
		node.data.Timestamp = timestamp
		// 重新插入
		lb.insertNode(node)
	} else {
		// 创建新节点
		playerData := PlayerData{
			PlayerId:  playerId,
			Score:     incrScore,
			Timestamp: timestamp,
		}
		lb.insertPlayerData(playerData)
	}
}

// insertPlayerData 插入新玩家数据
func (lb *Leaderboard) insertPlayerData(data PlayerData) {
	level := lb.randomLevel()
	node := &LeaderboardNode{
		data:     data,
		levels:   make([]*LeaderboardLevel, level),
		backward: nil,
	}

	for i := 0; i < level; i++ {
		node.levels[i] = &LeaderboardLevel{}
	}

	lb.insertNode(node)
	lb.playerMap[data.PlayerId] = node
	lb.length++
}

// insertNode 插入节点到跳表
func (lb *Leaderboard) insertNode(node *LeaderboardNode) {
	update := make([]*LeaderboardNode, lb.maxLevel)
	rank := make([]int, lb.maxLevel)

	// 从最高层开始查找插入位置
	x := lb.header
	for i := lb.level - 1; i >= 0; i-- {
		if i == lb.level-1 {
			rank[i] = 0
		} else {
			rank[i] = rank[i+1]
		}

		for x.levels[i].forward != nil && lb.compareNodes(x.levels[i].forward.data, node.data) {
			rank[i] += x.levels[i].span
			x = x.levels[i].forward
		}
		update[i] = x
	}

	// 更新层级
	level := len(node.levels)
	if level > lb.level {
		for i := lb.level; i < level; i++ {
			update[i] = lb.header
			update[i].levels[i].span = lb.length
		}
		lb.level = level
	}

	// 插入节点
	x = node
	for i := 0; i < level; i++ {
		x.levels[i].forward = update[i].levels[i].forward
		update[i].levels[i].forward = x

		x.levels[i].span = update[i].levels[i].span - (rank[0] - rank[i])
		update[i].levels[i].span = (rank[0] - rank[i]) + 1
	}

	// 设置backward指针
	if update[0] != lb.header {
		x.backward = update[0]
	}
	if x.levels[0].forward != nil {
		x.levels[0].forward.backward = x
	}
}

// deleteNode 从跳表删除节点
func (lb *Leaderboard) deleteNode(node *LeaderboardNode) {
	update := make([]*LeaderboardNode, lb.maxLevel)

	// 查找前驱节点
	x := lb.header
	for i := lb.level - 1; i >= 0; i-- {
		for x.levels[i].forward != nil && lb.compareNodes(x.levels[i].forward.data, node.data) && x.levels[i].forward != node {
			x = x.levels[i].forward
		}
		update[i] = x
	}

	// 删除节点
	for i := 0; i < lb.level; i++ {
		if update[i].levels[i].forward == node {
			update[i].levels[i].span += node.levels[i].span - 1
			update[i].levels[i].forward = node.levels[i].forward
		} else {
			update[i].levels[i].span--
		}
	}

	// 更新backward指针
	if node.levels[0].forward != nil {
		node.levels[0].forward.backward = node.backward
	}

	// 调整层级
	for lb.level > 1 && lb.header.levels[lb.level-1].forward == nil {
		lb.level--
	}

	lb.length--
}

// GetPlayerRank 获取玩家排名
func (lb *Leaderboard) GetPlayerRank(playerId string) *model.RankInfo {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	node, exists := lb.playerMap[playerId]
	if !exists {
		return nil
	}

	rank := lb.getPlayerRankInternal(node)
	return &model.RankInfo{
		PlayerId:  playerId,
		Score:     node.data.Score,
		Rank:      int64(rank),
		Timestamp: node.data.Timestamp,
	}
}

// getPlayerRankInternal 内部获取排名函数（不加锁）
func (lb *Leaderboard) getPlayerRankInternal(node *LeaderboardNode) int {
	rank := 0
	x := lb.header
	for i := lb.level - 1; i >= 0; i-- {
		for x.levels[i].forward != nil && lb.compareNodes(x.levels[i].forward.data, node.data) && x.levels[i].forward != node {
			rank += x.levels[i].span
			x = x.levels[i].forward
		}
		if x.levels[i].forward == node {
			rank += x.levels[i].span
			return rank
		}
	}
	return 0
}

// GetTopN 获取前N名玩家
func (lb *Leaderboard) GetTopN(n int64) []*model.RankInfo {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	result := make([]*model.RankInfo, 0, n)
	x := lb.header.levels[0].forward
	rank := int64(1)

	for x != nil && rank <= n {
		result = append(result, &model.RankInfo{
			PlayerId:  x.data.PlayerId,
			Score:     x.data.Score,
			Rank:      rank,
			Timestamp: x.data.Timestamp,
		})
		x = x.levels[0].forward
		rank++
	}

	return result
}

// GetPlayerRankRange 获取玩家周边排名
func (lb *Leaderboard) GetPlayerRankRange(playerId string, rangeN int64) []*model.RankInfo {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	node, exists := lb.playerMap[playerId]
	if !exists {
		return nil
	}

	// 获取玩家排名
	playerRank := lb.getPlayerRankInternal(node)
	if playerRank == 0 {
		return nil
	}

	// 计算范围
	startRank := playerRank - int(rangeN/2)
	if startRank < 1 {
		startRank = 1
	}

	endRank := playerRank + int(rangeN/2)
	if endRank > lb.length {
		endRank = lb.length
	}

	result := make([]*model.RankInfo, 0, endRank-startRank+1)

	// 定位到起始排名
	x := lb.header.levels[0].forward
	currentRank := 1

	// 移动到起始位置
	for x != nil && currentRank < startRank {
		x = x.levels[0].forward
		currentRank++
	}

	// 收集结果
	for x != nil && currentRank <= endRank {
		result = append(result, &model.RankInfo{
			PlayerId:  x.data.PlayerId,
			Score:     x.data.Score,
			Rank:      int64(currentRank),
			Timestamp: x.data.Timestamp,
		})
		x = x.levels[0].forward
		currentRank++
	}

	return result
}

// GetPlayerRankDense 获取玩家密集排名
func (lb *Leaderboard) GetPlayerRankDense(playerId string) *model.RankInfo {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	node, exists := lb.playerMap[playerId]
	if !exists {
		return nil
	}

	// 计算密集排名
	rank := 1
	prevScore := float64(-1)
	x := lb.header.levels[0].forward

	for x != nil {
		if x.data.Score != prevScore {
			// 新分数，排名递增
			prevScore = x.data.Score
		}

		if x.data.PlayerId == playerId {
			return &model.RankInfo{
				PlayerId:  playerId,
				Score:     node.data.Score,
				Rank:      int64(rank),
				Timestamp: node.data.Timestamp,
			}
		}

		// 只有当分数不同时才增加排名
		if x.data.Score != prevScore {
			rank++
		}

		x = x.levels[0].forward
	}

	return nil
}

// GetTopNDense 获取密集排名的前N名
func (lb *Leaderboard) GetTopNDense(n int64) []*model.RankInfo {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	result := make([]*model.RankInfo, 0, n)
	x := lb.header.levels[0].forward
	rank := 0
	prevScore := float64(-1)

	for x != nil && len(result) < int(n) {
		if x.data.Score != prevScore {
			rank++
			prevScore = x.data.Score
		}

		result = append(result, &model.RankInfo{
			PlayerId:  x.data.PlayerId,
			Score:     x.data.Score,
			Rank:      int64(rank),
			Timestamp: x.data.Timestamp,
		})

		x = x.levels[0].forward
	}

	return result
}

// Initialize 初始化随机种子
func init() {
	rand.Seed(time.Now().UnixNano())
}
