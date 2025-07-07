CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- Or BIGSERIAL if you prefer auto-incrementing integers
    name VARCHAR(255) NOT NULL UNIQUE,
    image TEXT, -- URL to the image
    price NUMERIC(10, 2) NOT NULL,
    qty INT NOT NULL DEFAULT 0 CHECK (qty >= 0),
    out_of_stock BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for faster lookups on name if not using it as primary key
-- CREATE UNIQUE INDEX idx_products_name ON products (name);

-- Function to update updated_at automatically
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_products_updated_at
BEFORE UPDATE ON products
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

-- Optional: Function to update out_of_stock based on qty
CREATE OR REPLACE FUNCTION update_out_of_stock_status()
RETURNS TRIGGER AS $$
BEGIN
    NEW.out_of_stock = (NEW.qty = 0);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER set_out_of_stock_on_qty_change
BEFORE INSERT OR UPDATE OF qty ON products
FOR EACH ROW
EXECUTE FUNCTION update_out_of_stock_status();