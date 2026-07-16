INSERT INTO moderation_rules (rule_id, rule_name, rule_type, rule_pattern, threshold, action, risk_category, enabled, priority)
SELECT * FROM (VALUES
    ('rule-001', '色情内容检测', 'KEYWORD', '色情|色情图片|成人内容|sex|porn', 0.8, 'BLOCK', '色情', TRUE, 10),
    ('rule-002', '暴力内容检测', 'KEYWORD', '暴力|杀人|血腥|恐怖|violence|murder', 0.8, 'BLOCK', '暴力', TRUE, 10),
    ('rule-003', '仇恨言论检测', 'KEYWORD', '仇恨|歧视|种族|歧视|hate|racist', 0.8, 'BLOCK', '仇恨', TRUE, 10),
    ('rule-004', '垃圾广告检测', 'KEYWORD', '广告|推广|营销|促销|advertisement|spam', 0.7, 'REVIEW', '广告', TRUE, 20),
    ('rule-005', '敏感政治内容', 'KEYWORD', '政治|政府|领导人|敏感事件', 0.8, 'REVIEW', '政治', TRUE, 15),
    ('rule-006', '个人信息保护', 'REGEX', '[\\u4e00-\\u9fa5]{2,4}[的]?[身份证|手机号|电话|邮箱|住址]', 0.7, 'BLOCK', '隐私', TRUE, 5),
    ('rule-007', '未成年人保护', 'KEYWORD', '未成年人|儿童|小孩|minor|child', 0.7, 'REVIEW', '未成年人', TRUE, 8)
) AS tmp
WHERE NOT EXISTS (SELECT 1 FROM moderation_rules);