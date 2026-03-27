-- zhi-id-gen-go compatible schema
-- Kept aligned with the Java id-generator project.

CREATE TABLE IF NOT EXISTS leaf_alloc (
    biz_tag VARCHAR(128) PRIMARY KEY,
    max_id BIGINT NOT NULL DEFAULT 1,
    step INT NOT NULL DEFAULT 1000,
    description VARCHAR(256),
    update_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    version BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_leaf_alloc_update_time ON leaf_alloc(update_time);

INSERT INTO leaf_alloc (biz_tag, max_id, step, description)
VALUES
    ('default', 1, 1000, 'Default business tag'),
    ('user', 1, 2000, 'User ID sequence'),
    ('order', 1, 5000, 'Order ID sequence'),
    ('message', 1, 10000, 'Message ID sequence')
ON CONFLICT (biz_tag) DO NOTHING;

CREATE TABLE IF NOT EXISTS worker_id_alloc (
    worker_id   INTEGER     PRIMARY KEY CHECK (worker_id >= 0 AND worker_id <= 31),
    instance_id VARCHAR(64) NOT NULL DEFAULT '',
    lease_time  TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status      VARCHAR(16) NOT NULL DEFAULT 'released' CHECK (status IN ('active', 'released'))
);

INSERT INTO worker_id_alloc (worker_id, instance_id, lease_time, status)
VALUES
    (0,  '', CURRENT_TIMESTAMP, 'released'),
    (1,  '', CURRENT_TIMESTAMP, 'released'),
    (2,  '', CURRENT_TIMESTAMP, 'released'),
    (3,  '', CURRENT_TIMESTAMP, 'released'),
    (4,  '', CURRENT_TIMESTAMP, 'released'),
    (5,  '', CURRENT_TIMESTAMP, 'released'),
    (6,  '', CURRENT_TIMESTAMP, 'released'),
    (7,  '', CURRENT_TIMESTAMP, 'released'),
    (8,  '', CURRENT_TIMESTAMP, 'released'),
    (9,  '', CURRENT_TIMESTAMP, 'released'),
    (10, '', CURRENT_TIMESTAMP, 'released'),
    (11, '', CURRENT_TIMESTAMP, 'released'),
    (12, '', CURRENT_TIMESTAMP, 'released'),
    (13, '', CURRENT_TIMESTAMP, 'released'),
    (14, '', CURRENT_TIMESTAMP, 'released'),
    (15, '', CURRENT_TIMESTAMP, 'released'),
    (16, '', CURRENT_TIMESTAMP, 'released'),
    (17, '', CURRENT_TIMESTAMP, 'released'),
    (18, '', CURRENT_TIMESTAMP, 'released'),
    (19, '', CURRENT_TIMESTAMP, 'released'),
    (20, '', CURRENT_TIMESTAMP, 'released'),
    (21, '', CURRENT_TIMESTAMP, 'released'),
    (22, '', CURRENT_TIMESTAMP, 'released'),
    (23, '', CURRENT_TIMESTAMP, 'released'),
    (24, '', CURRENT_TIMESTAMP, 'released'),
    (25, '', CURRENT_TIMESTAMP, 'released'),
    (26, '', CURRENT_TIMESTAMP, 'released'),
    (27, '', CURRENT_TIMESTAMP, 'released'),
    (28, '', CURRENT_TIMESTAMP, 'released'),
    (29, '', CURRENT_TIMESTAMP, 'released'),
    (30, '', CURRENT_TIMESTAMP, 'released'),
    (31, '', CURRENT_TIMESTAMP, 'released')
ON CONFLICT (worker_id) DO NOTHING;

