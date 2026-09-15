CREATE TABLE IF NOT EXISTS fixed_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id VARCHAR(50) NOT NULL,
    tag VARCHAR(50) NOT NULL UNIQUE,
    serial_number VARCHAR(100) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    location VARCHAR(120) NOT NULL DEFAULT '',
    acquisition_date DATE NOT NULL,
    acquisition_cost NUMERIC(14,4) NOT NULL,
    residual_value NUMERIC(14,4) NOT NULL DEFAULT 0,
    useful_life_months INT NOT NULL,
    depreciation_method VARCHAR(30) NOT NULL DEFAULT 'STRAIGHT_LINE',
    accumulated_depreciation NUMERIC(14,4) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS asset_movements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id UUID NOT NULL REFERENCES fixed_assets(id),
    movement_type VARCHAR(20) NOT NULL,
    from_location VARCHAR(120) NOT NULL DEFAULT '',
    to_location VARCHAR(120) NOT NULL DEFAULT '',
    amount NUMERIC(14,4) NOT NULL DEFAULT 0,
    notes TEXT NOT NULL DEFAULT '',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_asset_movements_asset ON asset_movements(asset_id);
