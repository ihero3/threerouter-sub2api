-- 238_add_media_tasks_media_urls.sql
-- 多张产物 URL 全量落库：图片 n>1 时创建响应与轮询接口都能返回完整 urls。
-- 此前多张 URL 仅内存传递，轮询 GET /v1/media/:id 只能返回首张。
ALTER TABLE media_tasks ADD COLUMN IF NOT EXISTS media_urls JSONB;
