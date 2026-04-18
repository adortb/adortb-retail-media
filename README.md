# adortb-retail-media

零售媒体网络（Retail Media Network）广告服务，提供 SP（赞助商品）和 SB（品牌推广）广告投放、搜索/分类页拍卖、关键词匹配以及 ACOS/ROAS 等效果指标上报。

## 整体架构

```
              ┌─────────────────────────────────────────┐
              │     Web SDK / iOS / Android / CTV       │
              └────────────────┬────────────────────────┘
                               ↓
                       ┌───────────────┐
                       │   ADX Core    │◀──外部 DSP─┐
                       └───────┬───────┘            │
                   ┌───────────┼───────────┐        │
                   ↓           ↓           ↓        │
              ┌────────┐ ┌────────┐ ┌────────┐     │
              │  DSP   │ │  MMP   │ │  SSAI  │─────┘
              └───┬────┘ └───┬────┘ └────────┘
                  ↓          ↓
              ┌───────────────────────────┐
              │  Event Pipeline (Kafka)   │
              └───────┬───────────────────┘
        ┌─────────────┼─────────────┐
        ↓             ↓             ↓
  ┌─────────┐   ┌──────────┐   ┌──────────┐
  │ Billing │   │   DMP    │   │   CDP    │
  └─────────┘   └──────────┘   └──────────┘

  ┌─────────────────────────────────────────────┐
  │  ★ Retail Media（零售媒体网络 SP/SB/ACOS） │
  └─────────────────────────────────────────────┘
```

**Retail Media** 作为独立的垂直模块运行，不依赖 ADX Core 的竞价管道。广告主直接通过本服务管理活动并调用拍卖 API，结算数据推送给 Billing 服务，效果数据供 Admin Dashboard 消费。

## 技术栈

| 项目 | 说明 |
|------|------|
| Go 1.25.3 | 仅使用标准库 `net/http`，无第三方框架 |
| PostgreSQL | 驱动 `lib/pq v1.12.3` |
| HTTP 端口 | 8095（`PORT` 环境变量） |

## 目录结构

```
adortb-retail-media/
├── cmd/retail-media/main.go
├── internal/
│   ├── api/handler.go           # HTTP 路由（商品/SP/SB/拍卖/事件）
│   ├── catalog/product.go       # 商品目录
│   ├── catalog/inventory.go     # 库存管理
│   ├── catalog/category.go      # 类目树（路径表示法）
│   ├── campaign/sp_campaign.go  # SP 四层: Campaign→AdGroup→Keyword→ProductAd
│   ├── campaign/sb_campaign.go  # SB 活动（品牌推广）
│   ├── campaign/bid_strategy.go # CPC 手动 / CPA 自动出价
│   ├── auction/search_auction.go      # 搜索页拍卖（Vickrey 二价）
│   ├── auction/category_auction.go    # 分类页拍卖
│   ├── search/query_parser.go   # 搜索词解析（引号/排除词/类目限定）
│   ├── search/keyword_match.go  # exact/phrase/broad 匹配
│   ├── search/ranker.go         # 质量分数计算
│   └── reporting/metrics.go     # ACOS/ROAS/TACoS/AvgCPC
└── migrations/001_rmn.up.sql
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DATABASE_URL` | `postgres://localhost/adortb_rmn?sslmode=disable` | PostgreSQL 连接串 |
| `PORT` | `8095` | HTTP 监听端口 |

## HTTP API 概览

### 商品目录

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/products` | 列表（支持 `advertiser_id`、`category_id` 过滤） |
| POST | `/v1/products` | 创建/更新商品 |
| GET | `/v1/products/{sku}` | 查询单个 SKU |
| PUT | `/v1/products/{sku}` | 更新 SKU |
| DELETE | `/v1/products/{sku}` | 删除 SKU |
| POST | `/v1/products/import` | 批量导入 |

### SP 活动（四层结构）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/v1/campaigns/sp` | 列表 / 创建 |
| GET/PUT/DELETE | `/v1/campaigns/sp/{id}` | 查询 / 更新 / 删除 |
| GET | `/v1/campaigns/sp/{id}/performance` | ACOS / ROAS 指标 |
| POST | `/v1/ad-groups` | 创建广告组 |
| POST | `/v1/keywords` | 创建关键词 |
| POST | `/v1/product-ads` | 创建商品广告 |

### SB 活动

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/v1/campaigns/sb` | 列表 / 创建品牌推广活动 |

### 拍卖

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/ads/search` | 搜索页广告，返回 top N |
| POST | `/v1/ads/category` | 分类页广告 |

### 事件

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/events/click` | 点击事件上报 |
| POST | `/v1/events/purchase` | 购买事件上报 |
| GET | `/health` | 健康检查 |

## 核心特性

### SP 四层结构
```
Campaign → AdGroup → Keyword → ProductAd
```
每层独立控制预算、出价和状态。

### 拍卖机制（Vickrey 二价）
- 质量分数: `QS = CTR历史 × (0.5 + rating/5) × min(2.0, avgPrice/price)`
- 排序分数: `Rank = effectiveBid × QS`
- 成交价格: `CPC = nextBid + $0.01`

### 出价策略
- **CPC 手动出价**: 广告主直接设定每次点击出价
- **CPA 自动出价**: `effectiveBid = targetCPA × estimatedCVR`，受 `maxBid` 约束

### 关键词匹配得分
| 匹配类型 | 分数 |
|----------|------|
| exact    | 3 |
| phrase   | 2 |
| broad    | 1 |

### 预算守卫
使用 16 分片 buffered channel 实现并发安全的预算消耗，避免超支。

## 与其他服务的关系

| 方向 | 服务 | 说明 |
|------|------|------|
| 上游 | 零售商品系统 | 商品目录导入 |
| 上游 | 广告主管理后台 | 活动管理 |
| 下游 | Billing | 广告花费结算 |
| 下游 | Admin Dashboard | ACOS/ROAS 报表展示 |
| 消费方 | 零售媒体前端（电商页面） | 调用拍卖 API 获取广告位 |

## 快速启动

```bash
# 初始化数据库
psql $DATABASE_URL < migrations/001_rmn.up.sql

# 启动服务
DATABASE_URL=postgres://localhost/adortb_rmn?sslmode=disable PORT=8095 go run ./cmd/retail-media
```

## 文档

- [架构详情](docs/architecture.md) — SP 四层架构图、搜索拍卖流程、出价策略对比、指标计算公式
