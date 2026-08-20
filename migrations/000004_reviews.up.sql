ALTER TABLE reviews
    ADD COLUMN IF NOT EXISTS order_id TEXT REFERENCES orders (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS user_id TEXT REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS photos TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS seller_reply TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS seller_reply_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS reviews_product_idx ON reviews (product_id);
CREATE INDEX IF NOT EXISTS reviews_user_idx ON reviews (user_id);

CREATE UNIQUE INDEX IF NOT EXISTS reviews_user_product_uidx
    ON reviews (user_id, product_id)
    WHERE user_id IS NOT NULL;
