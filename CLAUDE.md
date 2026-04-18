# adortb-retail-media — Claude 开发指南

## 项目概述

零售媒体网络（Retail Media Network）广告服务。提供 SP（赞助商品）/SB（品牌推广）广告管理、搜索/分类页 Vickrey 二价拍卖、关键词匹配以及 ACOS/ROAS 效果指标上报。

- 语言: **Go 1.25.3**，仅使用标准库 `net/http`，禁止引入 Web 框架
- 数据库: **PostgreSQL**，驱动 `lib/pq v1.12.3`
- 监听端口: **8095**（`PORT` 环境变量）

## 代码结构速查

```
internal/
├── api/handler.go           # 所有 HTTP 路由的入口，只做参数解析和响应序列化
├── catalog/                 # 商品目录、库存、类目树
├── campaign/                # SP 四层 + SB 活动 + 出价策略
├── auction/                 # 搜索页 / 分类页拍卖（Vickrey 二价）
├── search/                  # 搜索词解析 / 关键词匹配 / 质量分数
└── reporting/               # ACOS / ROAS / TACoS / AvgCPC 指标聚合
```

## 关键业务规则（必须遵守）

### SP 四层结构
Campaign → AdGroup → Keyword → ProductAd，每层独立控制预算、出价和状态。
修改任何一层时必须检查父层状态；父层暂停则子层不得参与拍卖。

### 质量分数公式
```
QS = CTR历史 × (0.5 + rating/5) × min(2.0, avgPrice/price)
Rank = effectiveBid × QS
CPC = nextBid + 0.01   // Vickrey 二价
```
不得随意修改系数，如需调整须同步更新 `docs/architecture.md`。

### 出价策略
- CPC 手动: 直接使用广告主设定出价
- CPA 自动: `effectiveBid = targetCPA × estimatedCVR`，必须受 `maxBid` 上界约束

### 预算守卫
`campaign/bid_strategy.go` 使用 **16 分片 buffered channel** 实现并发安全的预算消耗。
禁止使用全局锁替代分片方案；修改时需保持分片数为 2 的幂次。

### 关键词匹配得分
exact=3 / phrase=2 / broad=1；得分用于候选集筛选，不参与 Rank 计算。

## 数据库约定

| 表 | 主键 | 备注 |
|----|------|------|
| `rmn_products` | `sku` | `keywords TEXT[]` + `attributes JSONB`，建有 GIN 索引 |
| `rmn_categories` | `id` | `path` 使用路径表示法（如 `/electronics/phones`） |
| `rmn_performance` | `(date, campaign_id)` | 每日聚合，不做实时行级更新 |

所有 DDL 变更必须通过 `migrations/` 目录下的迁移文件执行，禁止直接修改表结构。

## 开发规范

### 编写新功能前
1. 阅读 `internal/api/handler.go` 确认路由注册方式（标准库 `ServeMux`）
2. 阅读对应 `internal/<domain>/` 下已有文件，保持接口风格一致
3. 查看 `migrations/001_rmn.up.sql` 确认表结构

### 错误处理
- Handler 层统一返回 JSON `{"error": "..."}` 并设置对应 HTTP 状态码
- 内部函数返回 `error`，禁止在非 main 包使用 `log.Fatal`/`os.Exit`
- 数据库操作必须处理 `sql.ErrNoRows`，返回 404 而非 500

### 并发安全
- 任何共享状态必须通过 `sync.Mutex`、`sync.RWMutex` 或 channel 保护
- 预算相关操作使用已有的 16 分片方案，不得引入新的全局锁
- 拍卖流程本身是无状态的，每次请求独立计算

### 单元测试
- 每个包必须有对应 `_test.go` 文件
- 拍卖逻辑（`auction/`）和质量分数（`search/ranker.go`）需覆盖边界值
- 使用 `database/sql` 接口 mock 数据库，禁止在单元测试中连接真实数据库

### 性能注意事项
- 拍卖请求（`/v1/ads/search`、`/v1/ads/category`）是高频热路径，避免不必要的内存分配
- `rmn_products` 的关键词检索依赖 GIN 索引，查询时必须使用 `@>` 或 `&&` 操作符
- 上报事件（`/v1/events/*`）应异步写入，不阻塞响应

## 常用命令

```bash
# 运行所有测试
go test ./...

# 运行指定包测试（详细输出）
go test -v ./internal/auction/...

# 数据库迁移
psql $DATABASE_URL < migrations/001_rmn.up.sql

# 启动服务
DATABASE_URL=postgres://localhost/adortb_rmn?sslmode=disable PORT=8095 go run ./cmd/retail-media

# 构建二进制
go build -o bin/retail-media ./cmd/retail-media
```

## 相关文档

- [README.md](README.md) — 项目概述与 API 列表
- [docs/architecture.md](docs/architecture.md) — SP 四层架构图、拍卖流程、指标公式
