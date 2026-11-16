-- starfuyu_daily 当日全局星象/开篇总述（图片版软文顶部两块，spec 2026-11-16 §3.1）。
-- 每日一条，与 fortune（每星座一行）正交；tenant_id 恒 0 与内容表同精神。
CREATE TABLE starfuyu_daily (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id  BIGINT       NOT NULL DEFAULT 0,
    date       VARCHAR(10)  NOT NULL,
    astro      VARCHAR(64)  NOT NULL DEFAULT '',
    intro      TEXT,
    created_at DATETIME     NULL,
    updated_at DATETIME     NULL,
    UNIQUE KEY uk_daily_tenant_date (tenant_id, date)
);
