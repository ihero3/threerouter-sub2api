package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/publicsuffix"
)

var registrationEmailDomainPattern = regexp.MustCompile(
	`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`,
)

// RegistrationEmailSuffix extracts normalized suffix in "@domain" form.
func RegistrationEmailSuffix(email string) string {
	_, domain, ok := splitEmailForPolicy(email)
	if !ok {
		return ""
	}
	return "@" + domain
}

// RegistrationEmailDomain 返回邮箱对应的可注册主域名，用于域名注册额度归一化。
// 例如 abc.com 和 abcd.abc.com 都返回 abc.com；无法从公共后缀表归一化时保留原域名。
func RegistrationEmailDomain(email string) string {
	_, domain, ok := splitEmailForPolicy(email)
	if !ok {
		return ""
	}
	return NormalizeRegistrationEmailDomain(domain)
}

// NormalizeRegistrationEmailDomain 将邮箱域名归一为可注册主域名。
func NormalizeRegistrationEmailDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(domain, "@")))
	domain = strings.TrimRight(domain, ".")
	if domain == "" {
		return ""
	}
	registrable, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return domain
	}
	return registrable
}

// IsRegistrationEmailSuffixAllowed checks whether an email is allowed by suffix whitelist.
// Empty whitelist means allow all.
func IsRegistrationEmailSuffixAllowed(email string, whitelist []string) bool {
	if len(whitelist) == 0 {
		return true
	}
	_, domain, ok := splitEmailForPolicy(email)
	if !ok {
		return false
	}
	suffix := "@" + domain
	for _, allowed := range whitelist {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if strings.HasPrefix(allowed, "@") && suffix == allowed {
			return true
		}
		if strings.HasPrefix(allowed, "*.") && registrationEmailDomainMatchesWildcard(domain, allowed) {
			return true
		}
	}
	return false
}

// IsRegistrationEmailDomainBlacklisted 判断邮箱是否命中黑名单。
// 支持两种条目：
//   - 域名条目（my.id / @my.id / *.my.id）：匹配该域名及其所有子域名；
//   - 完整邮箱条目（test@gmail.com）：只精确拦截该邮箱，不受白名单影响。
func IsRegistrationEmailDomainBlacklisted(email string, blacklist []string) bool {
	if len(blacklist) == 0 {
		return false
	}
	local, domain, ok := splitEmailForPolicy(email)
	if !ok {
		return false
	}
	normalizedEmail := local + "@" + domain
	for _, blocked := range blacklist {
		entry := strings.ToLower(strings.TrimSpace(blocked))
		if entry == "" {
			continue
		}
		// 完整邮箱：精确匹配单个地址。
		if !strings.HasPrefix(entry, "@") && !strings.HasPrefix(entry, "*.") && strings.Contains(entry, "@") {
			if entry == normalizedEmail {
				return true
			}
			continue
		}
		base := strings.TrimPrefix(entry, "@")
		base = strings.TrimPrefix(base, "*.")
		base = strings.TrimRight(base, ".")
		if base == "" || !isValidRegistrationEmailDomain(base) {
			continue
		}
		if domain == base || strings.HasSuffix(domain, "."+base) {
			return true
		}
	}
	return false
}

// NormalizeRegistrationEmailSuffixBlacklist normalizes and validates blacklist items.
// 条目可以是域名（my.id）、通配子域名（*.my.id）或完整邮箱（test@gmail.com）。
func NormalizeRegistrationEmailSuffixBlacklist(raw []string) ([]string, error) {
	return normalizeRegistrationEmailBlacklist(raw, true)
}

// ParseRegistrationEmailSuffixBlacklist parses persisted JSON into normalized entries.
// Invalid entries are ignored to keep old misconfigurations from breaking runtime reads.
func ParseRegistrationEmailSuffixBlacklist(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []string{}
	}
	normalized, _ := normalizeRegistrationEmailBlacklist(items, false)
	if len(normalized) == 0 {
		return []string{}
	}
	return normalized
}

// IsRegistrationEmailSuffixLimited 判断非空白名单是否对该邮箱域名启用单账户额度。
func IsRegistrationEmailSuffixLimited(email string, whitelist []string) bool {
	return len(whitelist) > 0 && !IsRegistrationEmailSuffixAllowed(email, whitelist)
}

// NormalizeRegistrationEmailSuffixWhitelist normalizes and validates suffix whitelist items.
func NormalizeRegistrationEmailSuffixWhitelist(raw []string) ([]string, error) {
	return normalizeRegistrationEmailSuffixWhitelist(raw, true)
}

// ParseRegistrationEmailSuffixWhitelist parses persisted JSON into normalized suffixes.
// Invalid entries are ignored to keep old misconfigurations from breaking runtime reads.
func ParseRegistrationEmailSuffixWhitelist(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []string{}
	}
	normalized, _ := normalizeRegistrationEmailSuffixWhitelist(items, false)
	if len(normalized) == 0 {
		return []string{}
	}
	return normalized
}

func normalizeRegistrationEmailSuffixWhitelist(raw []string, strict bool) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		normalized, err := normalizeRegistrationEmailSuffix(item)
		if err != nil {
			if strict {
				return nil, err
			}
			continue
		}
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func normalizeRegistrationEmailBlacklist(raw []string, strict bool) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		normalized, err := normalizeRegistrationEmailBlacklistEntry(item)
		if err != nil {
			if strict {
				return nil, err
			}
			continue
		}
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// normalizeRegistrationEmailBlacklistEntry 归一化黑名单条目。
// 完整邮箱保持 local@domain 形式（仅转小写），域名条目归一为 @domain / *.domain。
func normalizeRegistrationEmailBlacklistEntry(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "", nil
	}

	if !strings.HasPrefix(value, "*.") && !strings.HasPrefix(value, "@") && strings.Contains(value, "@") {
		if strings.Count(value, "@") != 1 || strings.ContainsAny(value, " \t") {
			return "", fmt.Errorf("invalid email blacklist entry: %q", raw)
		}
		local, domain, ok := splitEmailForPolicy(value)
		if !ok || !isValidRegistrationEmailDomain(domain) {
			return "", fmt.Errorf("invalid email blacklist entry: %q", raw)
		}
		return local + "@" + domain, nil
	}

	return normalizeRegistrationEmailSuffix(value)
}

func normalizeRegistrationEmailSuffix(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "", nil
	}

	if strings.HasPrefix(value, "*.") {
		domain := strings.TrimPrefix(value, "*.")
		if !isValidRegistrationEmailDomain(domain) {
			return "", fmt.Errorf("invalid email suffix: %q", raw)
		}
		return "*." + domain, nil
	}

	domain := value
	if strings.Contains(value, "@") {
		if !strings.HasPrefix(value, "@") || strings.Count(value, "@") != 1 {
			return "", fmt.Errorf("invalid email suffix: %q", raw)
		}
		domain = strings.TrimPrefix(value, "@")
	}

	if !isValidRegistrationEmailDomain(domain) {
		return "", fmt.Errorf("invalid email suffix: %q", raw)
	}

	return "@" + domain, nil
}

func isValidRegistrationEmailDomain(domain string) bool {
	return domain != "" &&
		!strings.Contains(domain, "@") &&
		registrationEmailDomainPattern.MatchString(domain)
}

func registrationEmailDomainMatchesWildcard(domain string, allowed string) bool {
	base := strings.TrimPrefix(allowed, "*.")
	if !isValidRegistrationEmailDomain(base) {
		return false
	}
	return domain == base || strings.HasSuffix(domain, "."+base)
}

func splitEmailForPolicy(raw string) (local string, domain string, ok bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	local, domain, found := strings.Cut(email, "@")
	if !found || local == "" || domain == "" || strings.Contains(domain, "@") {
		return "", "", false
	}
	domain = strings.TrimRight(domain, ".")
	if domain == "" {
		return "", "", false
	}
	return local, domain, true
}
