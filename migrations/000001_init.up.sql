CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    email         TEXT NOT NULL UNIQUE,
    phone         TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('buyer', 'seller', 'admin')),
    banned_at     TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE shops (
    id          TEXT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    shop_name   TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    logo        TEXT NOT NULL DEFAULT '',
    banner      TEXT NOT NULL DEFAULT '',
    city        TEXT NOT NULL DEFAULT 'Москва',
    phone       TEXT NOT NULL DEFAULT '',
    kind        TEXT NOT NULL DEFAULT 'shop' CHECK (kind IN ('shop', 'private')),
    delivery    TEXT[] NOT NULL DEFAULT ARRAY['cdek', 'pochta']::TEXT[],
    hidden      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE products (
    id                 TEXT PRIMARY KEY,
    title              TEXT NOT NULL,
    description        TEXT NOT NULL DEFAULT '',
    price_kopecks      BIGINT NOT NULL CHECK (price_kopecks > 0),
    old_price_kopecks  BIGINT,
    rating             NUMERIC(3, 2) NOT NULL DEFAULT 0,
    reviews_count      INT NOT NULL DEFAULT 0,
    seller_kind        TEXT NOT NULL CHECK (seller_kind IN ('shop', 'private')),
    condition          TEXT NOT NULL CHECK (condition IN ('new', 'used')),
    category           TEXT NOT NULL,
    image              TEXT NOT NULL,
    images             TEXT[] NOT NULL DEFAULT '{}',
    seller_id          TEXT NOT NULL REFERENCES shops (id) ON DELETE CASCADE,
    city               TEXT NOT NULL,
    weight_kg          NUMERIC(12, 3) NOT NULL DEFAULT 0.3,
    in_stock           INT NOT NULL DEFAULT 0,
    delivery           TEXT[] NOT NULL DEFAULT '{}',
    tags               TEXT[] NOT NULL DEFAULT '{}',
    trade_type         TEXT NOT NULL DEFAULT 'retail' CHECK (trade_type IN ('retail', 'wholesale', 'dropship')),
    hidden             BOOLEAN NOT NULL DEFAULT FALSE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX products_category_idx ON products (category);
CREATE INDEX products_seller_idx ON products (seller_id);
CREATE INDEX products_created_idx ON products (created_at DESC);

CREATE TABLE reviews (
    id         TEXT PRIMARY KEY,
    product_id TEXT NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    author     TEXT NOT NULL,
    rating     INT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    text       TEXT NOT NULL,
    date       TEXT NOT NULL
);

CREATE TABLE reels (
    id           TEXT PRIMARY KEY,
    product_id   TEXT NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    seller_id    TEXT NOT NULL REFERENCES shops (id) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    caption      TEXT NOT NULL DEFAULT '',
    likes        INT NOT NULL DEFAULT 0,
    duration_sec INT NOT NULL DEFAULT 18,
    gradient     TEXT[] NOT NULL DEFAULT ARRAY['#1c1917', '#e2571b']::TEXT[],
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE reel_comments (
    id         TEXT PRIMARY KEY,
    reel_id    TEXT NOT NULL REFERENCES reels (id) ON DELETE CASCADE,
    author     TEXT NOT NULL,
    text       TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE reel_likes (
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    reel_id TEXT NOT NULL REFERENCES reels (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, reel_id)
);

CREATE TABLE stories (
    id         TEXT PRIMARY KEY,
    seller_id  TEXT NOT NULL REFERENCES shops (id) ON DELETE CASCADE,
    image      TEXT NOT NULL,
    caption    TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE cart_items (
    user_id    TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    product_id TEXT NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    qty        INT NOT NULL CHECK (qty > 0),
    PRIMARY KEY (user_id, product_id)
);

CREATE TABLE favorites (
    user_id    TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    product_id TEXT NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, product_id)
);

CREATE TABLE orders (
    id                     TEXT PRIMARY KEY,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    buyer_id               TEXT NOT NULL REFERENCES users (id),
    seller_id              TEXT NOT NULL REFERENCES shops (id),
    city                   TEXT NOT NULL,
    address                TEXT NOT NULL,
    delivery               TEXT NOT NULL CHECK (delivery IN ('cdek', 'pochta', 'pickup')),
    delivery_price_kopecks BIGINT NOT NULL,
    eta                    TEXT NOT NULL DEFAULT '',
    track_number           TEXT,
    status                 TEXT NOT NULL,
    total_kopecks          BIGINT NOT NULL
);

CREATE INDEX orders_buyer_idx ON orders (buyer_id, created_at DESC);
CREATE INDEX orders_seller_idx ON orders (seller_id, created_at DESC);

CREATE TABLE order_items (
    id            BIGSERIAL PRIMARY KEY,
    order_id      TEXT NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    product_id    TEXT NOT NULL,
    title         TEXT NOT NULL,
    price_kopecks BIGINT NOT NULL,
    qty           INT NOT NULL,
    image         TEXT NOT NULL
);

CREATE TABLE order_events (
    id         BIGSERIAL PRIMARY KEY,
    order_id   TEXT NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    status     TEXT NOT NULL,
    note       TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE payments (
    id               UUID PRIMARY KEY,
    order_id         TEXT NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    amount_kopecks   BIGINT NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('pending', 'waiting_for_capture', 'succeeded', 'canceled')),
    confirmation_url TEXT,
    raw_json         JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX payments_order_idx ON payments (order_id);

CREATE TABLE chat_threads (
    id        TEXT PRIMARY KEY,
    seller_id TEXT NOT NULL REFERENCES shops (id) ON DELETE CASCADE,
    buyer_id  TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    UNIQUE (seller_id, buyer_id)
);

CREATE TABLE chat_messages (
    id         TEXT PRIMARY KEY,
    thread_id  TEXT NOT NULL REFERENCES chat_threads (id) ON DELETE CASCADE,
    from_id    TEXT NOT NULL,
    text       TEXT NOT NULL,
    read       BOOLEAN NOT NULL DEFAULT FALSE,
    hidden     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX chat_messages_thread_idx ON chat_messages (thread_id, created_at);

CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked    BOOLEAN NOT NULL DEFAULT FALSE
);
