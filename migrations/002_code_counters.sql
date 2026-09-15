CREATE TABLE code_counters (
    name TEXT PRIMARY KEY,
    n BIGINT NOT NULL DEFAULT 0
);

INSERT INTO code_counters (name, n)
SELECT 'asset_tag', COALESCE(MAX(tag::BIGINT), 0)
FROM fixed_assets
WHERE tag ~ '^[0-9]+$';
