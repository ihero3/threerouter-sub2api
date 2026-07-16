UPDATE moderation_rules
SET rule_name = CASE rule_id
    WHEN 'rule-001' THEN 'Pornographic Content Detection'
    WHEN 'rule-002' THEN 'Violent Content Detection'
    WHEN 'rule-003' THEN 'Hate Speech Detection'
    WHEN 'rule-004' THEN 'Spam & Advertising Detection'
    WHEN 'rule-005' THEN 'Sensitive Political Content'
    WHEN 'rule-006' THEN 'Personal Information Protection'
    WHEN 'rule-007' THEN 'Minor Protection'
    ELSE rule_name
END,
risk_category = CASE rule_id
    WHEN 'rule-001' THEN 'Pornography'
    WHEN 'rule-002' THEN 'Violence'
    WHEN 'rule-003' THEN 'Hate'
    WHEN 'rule-004' THEN 'Advertising'
    WHEN 'rule-005' THEN 'Political'
    WHEN 'rule-006' THEN 'Privacy'
    WHEN 'rule-007' THEN 'Minors'
    ELSE risk_category
END
WHERE rule_id IN ('rule-001', 'rule-002', 'rule-003', 'rule-004', 'rule-005', 'rule-006', 'rule-007')
  AND user_id IS NULL;
