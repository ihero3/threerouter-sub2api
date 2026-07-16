-- 将数组类型字段改为 JSONB 类型，支持 JSON 序列化存储

ALTER TABLE user_jurisdiction_mappings
    ALTER COLUMN applicable_regulations TYPE JSONB USING applicable_regulations::text::jsonb,
    ALTER COLUMN required_measures TYPE JSONB USING required_measures::text::jsonb,
    ALTER COLUMN recommended_actions TYPE JSONB USING recommended_actions::text::jsonb;
