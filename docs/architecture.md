# 架构详情

## SP 广告四层架构

SP（Sponsored Products）采用四层嵌套结构，每层独立管理预算、出价和启停状态。

```
┌─────────────────────────────────────────────────────┐
│  Campaign（活动）                                    │
│  - 总预算上限 daily_budget                          │
│  - 状态: ACTIVE / PAUSED / ARCHIVED                 │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │  AdGroup（广告组）                             │  │
│  │  - 组级默认出价 default_bid                   │  │
│  │  - 出价策略: CPC 手动 / CPA 自动              │  │
│  │                                               │  │
│  │  ┌─────────────────┐  ┌─────────────────┐    │  │
│  │  │  Keyword（关键词）│  │  Keyword        │    │  │
│  │  │  - 匹配类型      │  │  - 自定义出价   │    │  │
│  │  │  - 关键词出价    │  │                 │    │  │
│  │  └────────┬────────┘  └────────┬────────┘    │  │
│  │           │                    │             │  │
│  │  ┌────────▼────────────────────▼────────┐    │  │
│  │  │  ProductAd（商品广告）                │    │  │
│  │  │  - SKU 关联                          │    │  │
│  │  │  - 广告素材 / 落地页                 │    │  │
│  │  └──────────────────────────────────────┘    │  │
│  └───────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
```

### 状态继承规则

父层 PAUSED/ARCHIVED 时，子层不参与拍卖，无论子层自身状态如何。
生效优先级（从高到低）：Campaign → AdGroup → Keyword → ProductAd。

### 数据库映射

```
rmn_sp_campaigns    (id, advertiser_id, daily_budget, status, ...)
rmn_sp_ad_groups    (id, campaign_id, default_bid, bid_strategy, ...)
rmn_sp_keywords     (id, ad_group_id, keyword, match_type, bid, ...)
rmn_sp_product_ads  (id, ad_group_id, sku, status, ...)
```

---

## SB 品牌推广活动

SB（Sponsored Brands）是一层结构，支持 headline / logo / landing_page 以及关联多个 SKU。

```
rmn_sb_campaigns (id, advertiser_id, headline, logo_url, landing_page, skus VARCHAR[], ...)
```

---

## 搜索拍卖流程

```
请求: POST /v1/ads/search
        │
        ▼
┌───────────────────────┐
│  1. 搜索词解析         │
│  query_parser.go       │
│  - 引号精确短语        │
│  - 排除词 (-word)      │
│  - 类目限定 (cat:xxx)  │
└──────────┬────────────┘
           │
           ▼
┌───────────────────────┐
│  2. 关键词匹配         │
│  keyword_match.go      │
│  - exact  → 分数 3    │
│  - phrase → 分数 2    │
│  - broad  → 分数 1    │
│  → 生成候选广告集      │
└──────────┬────────────┘
           │
           ▼
┌───────────────────────────────────────────────────┐
│  3. 质量分数计算 (ranker.go)                       │
│                                                   │
│  QS = CTR历史 × (0.5 + rating/5) × price_factor  │
│                                                   │
│  price_factor = min(2.0, avgCategoryPrice/price)  │
│    → 商品定价越低于品类均价，质量加成越高（上限×2）│
│                                                   │
│  CTR历史: 过去 N 天点击率滚动平均                 │
│  rating:  商品评分 [0, 5]                         │
└──────────┬────────────────────────────────────────┘
           │
           ▼
┌───────────────────────────────────────────────────┐
│  4. 排序分数计算                                   │
│                                                   │
│  Rank = effectiveBid × QS                         │
│                                                   │
│  effectiveBid:                                    │
│    CPC 手动 → 广告主设定出价                       │
│    CPA 自动 → targetCPA × estimatedCVR            │
│               （受 maxBid 上界约束）               │
└──────────┬────────────────────────────────────────┘
           │
           ▼
┌───────────────────────────────────────────────────┐
│  5. Vickrey 二价拍卖                               │
│                                                   │
│  按 Rank 降序排列，取 top N                        │
│  CPC = 下一名出价 × (下一名QS / 本名QS) + $0.01   │
│                                                   │
│  简化形式（同 QS 场景）:                           │
│  CPC = nextBid + $0.01                            │
│                                                   │
│  赢家实际付费 ≤ 自身出价，激励真实报价             │
└──────────┬────────────────────────────────────────┘
           │
           ▼
┌───────────────────────┐
│  6. 预算守卫检查       │
│  16 分片 buffered ch   │
│  → 超预算广告剔除      │
└──────────┬────────────┘
           │
           ▼
        响应: top N 广告列表
```

---

## 出价策略对比

| 维度 | CPC 手动出价 | CPA 自动出价 |
|------|-------------|-------------|
| 控制方 | 广告主手动设定 | 系统自动计算 |
| 计算公式 | `effectiveBid = bid` | `effectiveBid = targetCPA × estimatedCVR` |
| 上界约束 | 无（由广告主自行控制） | `maxBid` 强制上界 |
| 适用场景 | 熟悉 CPC 市场价、精细控制 | 以目标转化成本为导向、自动优化 |
| 风险 | 出价过高导致 ROI 下降 | estimatedCVR 不准时偏差较大 |
| 依赖数据 | 无 | 历史 CVR 统计（`rmn_performance`） |

---

## ACOS / ROAS / TACoS / AvgCPC 计算公式

所有指标基于 `rmn_performance` 表的日聚合数据计算：

```
rmn_performance(date, campaign_id, impressions, clicks, spend, purchases, sales)
```

### CTR（点击率）
```
CTR = clicks / impressions
```

### CVR（转化率）
```
CVR = purchases / clicks
```

### ACOS（广告花费占比）
```
ACOS = spend / sales × 100%
```
含义: 每产生 1 元广告带来的销售额，需花费的广告费比例。值越低，广告效率越高。

### ROAS（广告投资回报率）
```
ROAS = sales / spend
```
含义: 每花费 1 元广告费带来的销售额。与 ACOS 互为倒数关系 `ROAS = 1 / ACOS`。

### TACoS（总广告花费占比）
```
TACoS = spend / totalSales × 100%
```
其中 `totalSales` 包含广告带动销售和自然销售之和。TACoS 比 ACOS 更能反映广告对整体业务的影响。

### AvgCPC（平均点击成本）
```
AvgCPC = spend / clicks
```

### 指标健康参考阈值

| 指标 | 健康范围 | 说明 |
|------|----------|------|
| ACOS | 15% ~ 30% | 品类差异较大，需结合毛利率判断 |
| ROAS | 3x ~ 7x | 对应 ACOS 14% ~ 33% |
| TACoS | 5% ~ 15% | 自然流量占比高时 TACoS 明显低于 ACOS |
| AvgCPC | 视品类而定 | 结合转化率判断是否划算 |
| CTR | 0.1% ~ 2% | 低于 0.1% 考虑优化素材或出价 |

---

## 预算守卫实现原理

```
16 个分片（shard 0 ~ 15）
每个广告活动按 campaignID % 16 路由到对应分片

shard[i]:
  ┌─────────────────────────────┐
  │  buffered channel (cap=1)   │
  │  + remainingBudget float64  │
  └─────────────────────────────┘

扣费流程:
  1. shard = campaignID % 16
  2. 向 shard[i].ch 发送令牌（获取锁）
  3. 检查 remainingBudget >= cost
  4. 若满足: remainingBudget -= cost，释放令牌，返回 true
  5. 若不满足: 释放令牌，返回 false（广告不参与本次拍卖）
```

16 分片将锁竞争降低至 1/16，适合同时存在大量并发拍卖请求的场景。
