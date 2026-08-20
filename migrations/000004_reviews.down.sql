DROP INDEX IF EXISTS reviews_user_product_uidx;
DROP INDEX IF EXISTS reviews_user_idx;
DROP INDEX IF EXISTS reviews_product_idx;

ALTER TABLE reviews
    DROP COLUMN IF EXISTS seller_reply_at,
    DROP COLUMN IF EXISTS seller_reply,
    DROP COLUMN IF EXISTS photos,
    DROP COLUMN IF EXISTS user_id,
    DROP COLUMN IF EXISTS order_id;
