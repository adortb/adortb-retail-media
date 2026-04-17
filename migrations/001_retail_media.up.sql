-- Retail Media Network 数据库表

-- 商品目录
CREATE TABLE rmn_products (
    sku VARCHAR(64) PRIMARY KEY,
    advertiser_id BIGINT NOT NULL,
    title VARCHAR(512) NOT NULL,
    category_id VARCHAR(64),
    brand VARCHAR(255),
    price DECIMAL(15,2),
    stock_level INT,
    image_url TEXT,
    product_url TEXT,
    rating DECIMAL(3,2),
    review_count INT,
    attributes JSONB,
    keywords TEXT[],
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_products_keywords ON rmn_products USING GIN (keywords);
CREATE INDEX idx_products_cat ON rmn_products(category_id);
CREATE INDEX idx_products_advertiser ON rmn_products(advertiser_id);

-- 类目树
CREATE TABLE rmn_categories (
    id VARCHAR(64) PRIMARY KEY,
    parent_id VARCHAR(64),
    name VARCHAR(255) NOT NULL,
    level INT DEFAULT 0,
    path VARCHAR(1024),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- SP 活动
CREATE TABLE rmn_sp_campaigns (
    id BIGSERIAL PRIMARY KEY,
    advertiser_id BIGINT NOT NULL,
    name VARCHAR(255),
    targeting_type VARCHAR(20) DEFAULT 'manual',
    daily_budget DECIMAL(15,2),
    start_date DATE,
    end_date DATE,
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_sp_campaigns_advertiser ON rmn_sp_campaigns(advertiser_id);

CREATE TABLE rmn_sp_ad_groups (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT REFERENCES rmn_sp_campaigns(id),
    name VARCHAR(255),
    default_bid DECIMAL(10,4),
    status VARCHAR(20) DEFAULT 'active'
);

CREATE TABLE rmn_sp_keywords (
    id BIGSERIAL PRIMARY KEY,
    ad_group_id BIGINT REFERENCES rmn_sp_ad_groups(id),
    keyword VARCHAR(255) NOT NULL,
    match_type VARCHAR(20) NOT NULL,
    bid DECIMAL(10,4),
    status VARCHAR(20) DEFAULT 'active'
);
CREATE INDEX idx_sp_keywords_adgroup ON rmn_sp_keywords(ad_group_id);

CREATE TABLE rmn_sp_product_ads (
    id BIGSERIAL PRIMARY KEY,
    ad_group_id BIGINT REFERENCES rmn_sp_ad_groups(id),
    sku VARCHAR(64) REFERENCES rmn_products(sku),
    status VARCHAR(20) DEFAULT 'active'
);

-- Sponsored Brand
CREATE TABLE rmn_sb_campaigns (
    id BIGSERIAL PRIMARY KEY,
    advertiser_id BIGINT NOT NULL,
    brand VARCHAR(255),
    headline VARCHAR(255),
    logo_url TEXT,
    landing_page TEXT,
    skus VARCHAR(64)[],
    daily_budget DECIMAL(15,2),
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_sb_campaigns_advertiser ON rmn_sb_campaigns(advertiser_id);

-- 成效数据
CREATE TABLE rmn_performance (
    date DATE NOT NULL,
    campaign_id BIGINT,
    ad_group_id BIGINT,
    keyword_id BIGINT,
    sku VARCHAR(64),
    impressions BIGINT DEFAULT 0,
    clicks BIGINT DEFAULT 0,
    spend DECIMAL(15,4) DEFAULT 0,
    purchases INT DEFAULT 0,
    sales DECIMAL(15,4) DEFAULT 0,
    PRIMARY KEY (date, campaign_id, ad_group_id, keyword_id, sku)
);
CREATE INDEX idx_performance_campaign ON rmn_performance(campaign_id, date);
