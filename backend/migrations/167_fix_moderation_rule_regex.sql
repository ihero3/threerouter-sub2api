UPDATE moderation_rules 
SET rule_pattern = '[\\x{4e00}-\\x{9fa5}]{2,4}[的]?[身份证|手机号|电话|邮箱|住址]'
WHERE rule_id = 'rule-006';
