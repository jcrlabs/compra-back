-- 001_init.sql

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE supermarket AS ENUM (
    'mercadona', 'froiz', 'gadis', 'carrefour', 'alcampo', 'eroski'
);

-- Users
CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT NOT NULL UNIQUE,
    password    TEXT NOT NULL,
    name        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Categories
CREATE TABLE IF NOT EXISTS categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    parent_id   UUID REFERENCES categories(id) ON DELETE SET NULL,
    icon        TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Products (normalised across supermarkets)
CREATE TABLE IF NOT EXISTS products (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    brand           TEXT,
    category_id     UUID REFERENCES categories(id) ON DELETE SET NULL,
    unit            TEXT NOT NULL DEFAULT 'unidad',
    unit_quantity   NUMERIC(10,3) NOT NULL DEFAULT 1,
    image_url       TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_products_category ON products(category_id);
CREATE INDEX IF NOT EXISTS idx_products_name ON products USING gin(to_tsvector('spanish', name));

-- Prices (one row per scrape event)
CREATE TABLE IF NOT EXISTS prices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id      UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    supermarket     supermarket NOT NULL,
    price           NUMERIC(10,2) NOT NULL,
    price_per_unit  NUMERIC(10,4),
    scraped_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    external_id     TEXT,
    external_url    TEXT
);

CREATE INDEX IF NOT EXISTS idx_prices_product_super ON prices(product_id, supermarket);
CREATE INDEX IF NOT EXISTS idx_prices_scraped_at    ON prices(scraped_at DESC);

-- View: latest price per product+supermarket
CREATE OR REPLACE VIEW current_prices AS
SELECT DISTINCT ON (product_id, supermarket)
    id, product_id, supermarket, price, price_per_unit, scraped_at, external_url
FROM prices
ORDER BY product_id, supermarket, scraped_at DESC;

-- Shopping lists
CREATE TABLE IF NOT EXISTS shopping_lists (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    share_token UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- List members (shared lists)
CREATE TABLE IF NOT EXISTS list_members (
    list_id     UUID NOT NULL REFERENCES shopping_lists(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (list_id, user_id)
);

-- List items
CREATE TABLE IF NOT EXISTS list_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    list_id     UUID NOT NULL REFERENCES shopping_lists(id) ON DELETE CASCADE,
    product_id  UUID REFERENCES products(id) ON DELETE SET NULL,
    name        TEXT NOT NULL,
    quantity    NUMERIC(10,3) NOT NULL DEFAULT 1,
    unit        TEXT,
    checked     BOOLEAN NOT NULL DEFAULT false,
    notes       TEXT,
    added_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_list_items_list ON list_items(list_id);

-- User supermarket preferences
CREATE TABLE IF NOT EXISTS user_supermarket_prefs (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    supermarket supermarket NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT true,
    PRIMARY KEY (user_id, supermarket)
);
