CREATE TABLE categories (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    parent_id   BIGINT REFERENCES categories(id),
    sort        INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE products (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug             TEXT NOT NULL UNIQUE,
    title            TEXT NOT NULL,
    brand            TEXT NOT NULL DEFAULT '',
    category_id      BIGINT NOT NULL REFERENCES categories(id),
    description      TEXT NOT NULL DEFAULT '',
    specs            JSONB NOT NULL DEFAULT '{}',
    base_price       NUMERIC(12,2) NOT NULL,
    compare_at_price NUMERIC(12,2),
    active           BOOLEAN NOT NULL DEFAULT TRUE,
    featured         BOOLEAN NOT NULL DEFAULT FALSE,
    seo_title        TEXT NOT NULL DEFAULT '',
    seo_description  TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_products_category ON products(category_id) WHERE active;
CREATE INDEX idx_products_featured ON products(featured) WHERE active AND featured;

CREATE TABLE product_variants (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id  BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sku         TEXT NOT NULL DEFAULT '',
    options     JSONB NOT NULL DEFAULT '{}',
    price       NUMERIC(12,2) NOT NULL,
    stock       INT NOT NULL DEFAULT 0,
    is_default  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_variants_product ON product_variants(product_id);

CREATE TABLE product_images (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id  BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_id  BIGINT REFERENCES product_variants(id) ON DELETE SET NULL,
    path        TEXT NOT NULL,
    alt         TEXT NOT NULL DEFAULT '',
    sort        INT NOT NULL DEFAULT 0
);
CREATE INDEX idx_images_product ON product_images(product_id);

CREATE TABLE carts (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id  TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE cart_items (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cart_id      BIGINT NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    variant_id   BIGINT NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    qty          INT NOT NULL CHECK (qty > 0),
    price_at_add NUMERIC(12,2) NOT NULL,
    UNIQUE (cart_id, variant_id)
);

CREATE TABLE orders (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code           TEXT NOT NULL UNIQUE,
    customer_name  TEXT NOT NULL,
    phone          TEXT NOT NULL,
    address        TEXT NOT NULL,
    city           TEXT NOT NULL,
    payment_method TEXT NOT NULL DEFAULT 'cod',
    status         TEXT NOT NULL DEFAULT 'pending',
    subtotal       NUMERIC(12,2) NOT NULL,
    shipping_fee   NUMERIC(12,2) NOT NULL DEFAULT 0,
    total          NUMERIC(12,2) NOT NULL,
    notes          TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_orders_phone ON orders(phone);
CREATE INDEX idx_orders_status ON orders(status);

CREATE TABLE order_items (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id   BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    variant_id BIGINT REFERENCES product_variants(id),
    title      TEXT NOT NULL,
    options    JSONB NOT NULL DEFAULT '{}',
    price      NUMERIC(12,2) NOT NULL,
    qty        INT NOT NULL CHECK (qty > 0)
);
CREATE INDEX idx_order_items_order ON order_items(order_id);

CREATE TABLE order_events (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id   BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    status     TEXT NOT NULL,
    note       TEXT NOT NULL DEFAULT '',
    actor      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_order_events_order ON order_events(order_id);

CREATE TABLE admin_users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'admin',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value JSONB NOT NULL
);
