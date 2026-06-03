-- 库存表
CREATE TABLE `inventory` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `inv_id` VARCHAR(64) NOT NULL COMMENT '库存ID，如 sku_id_warehouse_id',
  `sq` INT NOT NULL DEFAULT 0 COMMENT '可售库存',
  `wq` INT NOT NULL DEFAULT 0 COMMENT '预扣库存(下单未付款)',
  `oq` INT NOT NULL DEFAULT 0 COMMENT '占用库存(已付款未发货)',
  `lq` INT NOT NULL DEFAULT 0 COMMENT '预锁库存(用于Redis分桶)',
  `version` BIGINT NOT NULL DEFAULT 0 COMMENT '乐观锁版本号',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_inv_id` (`inv_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='库存表';

-- 锁库存单据表
CREATE TABLE `lock_order` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `lock_order_id` VARCHAR(64) NOT NULL COMMENT '锁库存单据ID',
  `inv_id` VARCHAR(64) NOT NULL COMMENT '库存ID',
  `lock_quantity` INT NOT NULL COMMENT '锁定数量',
  `bucket_index` INT NOT NULL COMMENT 'Redis分桶索引',
  `lock_status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态: 1=锁定中 2=已回收',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_lock_order_id` (`lock_order_id`),
  KEY `idx_inv_id` (`inv_id`),
  KEY `idx_inv_id_lock_status` (`inv_id`, `lock_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='锁库存单据表';

-- 库存扣减明细表
CREATE TABLE `inventory_deduct_detail` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `deduct_id` VARCHAR(64) NOT NULL COMMENT '扣减单据ID(幂等键)',
  `inv_id` VARCHAR(64) NOT NULL COMMENT '库存ID',
  `lock_order_id` VARCHAR(64) NOT NULL COMMENT '关联的锁库存单据ID',
  `order_id` VARCHAR(64) NOT NULL COMMENT '业务订单ID',
  `quantity` INT NOT NULL COMMENT '扣减数量',
  `deduct_status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态: 1=预扣 2=已确认 3=已取消',
  `deduct_type` TINYINT NOT NULL DEFAULT 1 COMMENT '扣减类型: 1=下单扣减 2=付款确认 3=订单取消',
  `extra_info` JSON DEFAULT NULL COMMENT '扩展信息',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_deduct_id` (`deduct_id`),
  KEY `idx_inv_id_lock_order_id_quantity` (`inv_id`, `lock_order_id`, `quantity`),
  KEY `idx_order_id` (`order_id`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='库存扣减明细表';