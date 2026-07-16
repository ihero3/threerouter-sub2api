-- Fix content moderation rules: change KEYWORD rules with pipe-separated patterns to REGEX
-- so that the '|' operator is treated as regex alternation rather than a literal string.
UPDATE moderation_rules
SET rule_type = 'REGEX'
WHERE rule_id IN (
    'rule-001',
    'rule-002',
    'rule-003',
    'rule-004',
    'rule-005',
    'rule-007'
)
AND rule_type = 'KEYWORD';
