# Go Game Leaderboard

一个高性能的游戏排行榜系统，支持百万级玩家的实时排名查询和更新。

## 项目结构

```
go-game-leaderboard/
├── cmd/
│   └── main.go              # 程序入口
├── internal/
│   ├── model/
│   │   └── rank.go          # 数据模型
│   ├── service/
│   │   └── leaderboard.go   # 本地内存版实现（跳表）
│   └── repository/
│       └── redis.go         # Redis 版本实现
├── go.mod
└── README.md
```

## 快速开始

```bash
# 运行测试
go run cmd/main.go
```
