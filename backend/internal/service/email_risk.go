package service

import (
	"context"
	"net"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ErrEmailRiskRejected 邮箱风险校验未通过：直接拒绝注册，且不会下发验证码。
var ErrEmailRiskRejected = infraerrors.BadRequest("EMAIL_RISK_REJECTED", "邮箱格式不符合注册要求")

// ErrTempMailMXRejected 邮箱域 MX 指向已知一次性邮箱服务集群：直接拒绝注册，不入库。
var ErrTempMailMXRejected = infraerrors.BadRequest("TEMP_MAIL_PROVIDER", "registration using temporary email providers is not allowed")

// registrationMXLookupTimeout 是一次性邮箱 MX 解析的超时。
// 解析器慢/卡死时 fail-open（放行），绝不拖垮注册/发码主流程。
const registrationMXLookupTimeout = 3 * time.Second

const (
	// 用户名中一旦出现即高危的符号（不含分隔符 . _ -）。
	highRiskLocalPartChars = "+#$&*=^`|~"
	// 允许出现的分隔符；只在连续重复或首尾出现并命中模板化命名时拦截。
	localPartSeparators = "._-"

	// 同一 IP 每小时允许注册的账号数上限。
	registrationIPHourlyLimit = 2
)

var (
	// 模板化命名：1-6 位字母 + 2-4 位结尾数字，例如 xxx1234、user8888、abcnn66。
	templateLocalPartPattern = regexp.MustCompile(`^[a-z]{1,6}[0-9]{2,4}$`)
	// 域名主体中出现连续 4 位以上数字，视为机器生成的异常域名。
	suspiciousDomainDigitsPattern = regexp.MustCompile(`[0-9]{4,}`)
)

// disposableEmailLabels 是临时邮箱/垃圾邮箱/异常域名的主体列表（不含 TLD）。
// 以主体名匹配，可同时拦截 tormails.com、tormails.net 等一批变体。
var disposableEmailLabels = map[string]struct{}{
	// 已知的临时邮箱/一次性邮箱服务
	"mailinator":         {},
	"maildrop":           {},
	"guerrillamail":      {},
	"guerrillamailblock": {},
	"sharklasers":        {},
	"grr":                {},
	"spam4":              {},
	"tempmail":           {},
	"tempmailo":          {},
	"temp-mail":          {},
	"10minutemail":       {},
	"yopmail":            {},
	"jetable":            {},
	"trashmail":          {},
	"throwaway":          {},
	"getnada":            {},
	"dispostable":        {},
	"fake-mail":          {},
	"fakemail":           {},
	"mailnesia":          {},
	"mintemail":          {},
	"mohmal":             {},
	"emailondeck":        {},
	"linshiyouxiang":     {},
	"88email":            {},
	"burnermail":         {},
	"tempail":            {},
	"emailfake":          {},
	// 业务中实际出现过的异常域名
	"tormails": {},
	"hastana":  {},
	"fldubai":  {},
}

// riskyLocalPartMashes 是常见键盘乱敲片段，命中即视为无规律字母拼接。
var riskyLocalPartMashes = []string{"asdf", "qwer", "zxcv", "hjkl", "qazwsx", "wsxedc"}

// ValidateRegistrationEmailRisk 校验注册邮箱是否命中高危规则。
// 必须在发送验证码之前调用；命中任意规则直接拒绝，不向该邮箱发信。
func ValidateRegistrationEmailRisk(email string) error {
	normalized := strings.TrimSpace(strings.ToLower(email))
	addr, err := mail.ParseAddress(normalized)
	if err != nil || addr.Address != normalized {
		return ErrEmailRiskRejected
	}
	at := strings.LastIndex(normalized, "@")
	if at <= 0 || at == len(normalized)-1 {
		return ErrEmailRiskRejected
	}
	local := normalized[:at]
	domain := normalized[at+1:]

	// 规则 1：用户名包含高危特殊符号（+、#、$、&、*、=、^、`、|、~）。
	if strings.ContainsAny(local, highRiskLocalPartChars) {
		return ErrEmailRiskRejected
	}

	// 分隔符：连续出现视为异常；首尾出现时仅在同时命中模板化命名时拦截，避免误杀正常邮箱。
	if hasRepeatedSeparator(local) {
		return ErrEmailRiskRejected
	}
	if startsOrEndsWithSeparator(local) && isTemplateOrMachineGenerated(local) {
		return ErrEmailRiskRejected
	}

	// 规则 2：明显机器生成格式的用户名。
	if isMachineGeneratedLocalPart(local) {
		return ErrEmailRiskRejected
	}

	// 规则 3：临时邮箱、垃圾邮箱、小众异常域名。
	if isRiskyEmailDomain(domain) {
		return ErrEmailRiskRejected
	}

	return nil
}

func hasRepeatedSeparator(local string) bool {
	for i := 1; i < len(local); i++ {
		if strings.ContainsRune(localPartSeparators, rune(local[i])) &&
			strings.ContainsRune(localPartSeparators, rune(local[i-1])) {
			return true
		}
	}
	return false
}

func startsOrEndsWithSeparator(local string) bool {
	if local == "" {
		return false
	}
	return strings.ContainsRune(localPartSeparators, rune(local[0])) ||
		strings.ContainsRune(localPartSeparators, rune(local[len(local)-1]))
}

// isMachineGeneratedLocalPart 判定用户名是否属于明显的机器生成格式。
func isMachineGeneratedLocalPart(local string) bool {
	return isTemplateOrMachineGenerated(local)
}

func isTemplateOrMachineGenerated(local string) bool {
	cleaned := strings.Trim(local, localPartSeparators)
	// 模板化命名：短字母串 + 末尾 2-4 位数字（不跨越分隔符，避免误杀 li.wei2024 这类正常拼音）。
	if templateLocalPartPattern.MatchString(cleaned) {
		return true
	}

	// 连续 6 位以上无元音字母，或 8 位以上元音占比过低，视为无规律字母拼接。
	for _, run := range letterRuns(cleaned) {
		if len(run) >= 6 && !containsVowel(run) {
			return true
		}
		if len(run) >= 8 && vowelRatio(run) < 0.25 {
			return true
		}
	}

	for _, mash := range riskyLocalPartMashes {
		if strings.Contains(cleaned, mash) {
			return true
		}
	}
	return false
}

// letterRuns 返回用户名中连续字母的片段。
func letterRuns(value string) []string {
	runs := make([]string, 0, 4)
	var current strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) {
			current.WriteRune(r)
			continue
		}
		if current.Len() > 0 {
			runs = append(runs, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		runs = append(runs, current.String())
	}
	return runs
}

func containsVowel(value string) bool {
	for _, r := range value {
		if strings.ContainsRune("aeiouy", r) {
			return true
		}
	}
	return false
}

func vowelRatio(value string) float64 {
	if value == "" {
		return 0
	}
	vowels := 0
	letters := 0
	for _, r := range value {
		letters++
		if strings.ContainsRune("aeiouy", r) {
			vowels++
		}
	}
	return float64(vowels) / float64(letters)
}

// isRiskyEmailDomain 判定域名是否属于临时邮箱/垃圾邮箱/小众异常域名。
func isRiskyEmailDomain(domain string) bool {
	if domain == "" {
		return true
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return true
	}
	for _, label := range labels {
		if label == "" {
			return true
		}
		if _, blocked := disposableEmailLabels[label]; blocked {
			return true
		}
	}
	root := labels[len(labels)-2]
	if suspiciousDomainDigitsPattern.MatchString(root) {
		return true
	}
	// 域名主体 8 位以上且完全没有元音，基本不会是正常企业域名。
	if len(root) >= 8 && !containsVowel(root) {
		return true
	}
	return false
}

// lookupTempMailMXCluster 解析邮箱域的 MX 记录，命中已知一次性邮箱服务集群则返回 true。
//
// best-effort、fail-open：DNS 解析失败、超时或域名无 MX 记录时返回 (false, nil)，
// 绝不因解析异常误杀正常注册。命中条件：邮箱域本身、或其任一 MX 主机域（含父域）
// 以 AffiliateRiskTempMailMXClusters 中任一后缀结尾。例如 x@foo.com 的 MX 为
// mail.temp-mail-pro.com 即命中——无需逐个收录 temp-mail 的下级域名，守株待兔。
func lookupTempMailMXCluster(ctx context.Context, email string) (bool, error) {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return false, nil
	}
	domain := strings.ToLower(strings.TrimSpace(email[at+1:]))
	if domain == "" {
		return false, nil
	}
	// 带超时的上下文做 DNS 解析：解析器慢/卡死时 fail-open，绝不拖垮注册/发码主流程。
	resolveCtx, cancel := context.WithTimeout(ctx, registrationMXLookupTimeout)
	defer cancel()
	mx, err := net.DefaultResolver.LookupMX(resolveCtx, domain)
	if err != nil {
		// 解析失败、超时或无 MX 记录：放行，避免把正常用户挡在门外。
		return false, nil
	}
	hosts := make([]string, 0, len(mx))
	for _, r := range mx {
		hosts = append(hosts, strings.TrimSuffix(strings.ToLower(r.Host), "."))
	}
	return tempMailMXClusterHit(domain, hosts, AffiliateRiskTempMailMXClusters), nil
}

// tempMailMXClusterHit 是 lookupTempMailMXCluster 的纯函数核心，便于单测。
// 邮箱域或任一 MX 主机域（含父域）命中 clusters 中任一后缀即返回 true。
func tempMailMXClusterHit(domain string, mxHosts, clusters []string) bool {
	match := func(host string) bool {
		host = strings.TrimSuffix(strings.ToLower(host), ".")
		for _, c := range clusters {
			if host == c || strings.HasSuffix(host, "."+c) {
				return true
			}
		}
		return false
	}
	if match(domain) {
		return true
	}
	for _, h := range mxHosts {
		if match(h) {
			return true
		}
	}
	return false
}
