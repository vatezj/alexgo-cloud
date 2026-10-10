-- 订单表（R4 裁定，T24 全页巡检撞 Error 1146 Table 'orders' doesn't exist）：
-- 列对齐 modules/order/model/order.go 的 Order 结构；CREATE IF NOT EXISTS 幂等（已存在即跳过）；不加外键。
CREATE TABLE IF NOT EXISTS `orders` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '用户ID',
  `product_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '商品ID',
  `amount`     BIGINT          NOT NULL DEFAULT 0 COMMENT '金额（分）',
  `status`     INT             NOT NULL DEFAULT 0 COMMENT '订单状态',
  `order_no`   VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '订单号',
  `created_at` DATETIME        NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME        NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_orders_order_no` (`order_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单表';
