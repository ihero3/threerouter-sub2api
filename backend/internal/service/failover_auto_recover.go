package service

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// FailoverAutoRecoverMessageTag 是写入 accounts.error_message 的自动恢复标记片段。
// 后台扫描任务只恢复带该标记的 error 账号，避免误恢复人工禁用或其他原因置错的账号。
const FailoverAutoRecoverMessageTag = "[auto-recover at "

// failoverAutoRecoverMaxAge 限制可接受的恢复时间上限，避免上游异常时间把账号长期锁死。
// 周维度配额窗口（7d）也在范围内。
const failoverAutoRecoverMaxAge = 8 * 24 * time.Hour

// failoverResetKeywordPattern 先在上游文案中定位 reset 提示位置，
// 再从该位置之后提取时间，避免误匹配消息中的其他日期。
var failoverResetKeywordPattern = regexp.MustCompile(`(?i)reset\s+(?:at|on)|重置`)

// failoverResetTimePattern 提取形如
//
//	2026-10-14 12:58:44 +0800 CST
//	2026-10-14T12:58:44+08:00
//	2026-10-14 12:58:44
//
// 的时间串。
var failoverResetTimePattern = regexp.MustCompile(
	`(\d{4}-\d{2}-\d{2})([ T])(\d{2}:\d{2}:\d{2})(?:(?:\s+|)(Z|[+-]\d{2}:?\d{2}))?(?:\s+([A-Za-z]{2,5}))?`,
)

// formatFailoverAutoRecoverTag 生成写入 error_message 的自动恢复标记。
func formatFailoverAutoRecoverTag(recoverAt time.Time) string {
	return FailoverAutoRecoverMessageTag + recoverAt.Format(time.RFC3339) + "]"
}

// failoverAutoRecoverAt 判断 failover 自动禁用是否应在配额重置后自动恢复。
// 仅用于 429：优先解析上游文案中「It will reset at ...」的明确时间，
// 退化到 Retry-After 响应头（相对秒数或 HTTP 时间）。
// 返回 nil 表示拿不到可信时间，调用方保持永久禁用行为。
func failoverAutoRecoverAt(statusCode int, upstreamMsg string, headers http.Header, now time.Time) *time.Time {
	if statusCode != http.StatusTooManyRequests {
		return nil
	}
	if recoverAt := parseUpstreamResetMessage(upstreamMsg, now); recoverAt != nil {
		return recoverAt
	}
	if recoverAt := parseRetryAfterResetTime(headers, now); recoverAt != nil && isValidFailoverAutoRecoverAt(*recoverAt, now) {
		return recoverAt
	}
	return nil
}

// parseUpstreamResetMessage 从上游错误文案中解析配额重置时间。
// 必须先出现 reset 关键字（含中文「重置」），防止误匹配消息中的其他日期。
func parseUpstreamResetMessage(msg string, now time.Time) *time.Time {
	trimmed := strings.TrimSpace(msg)
	if trimmed == "" {
		return nil
	}
	loc := failoverResetKeywordPattern.FindStringIndex(trimmed)
	if loc == nil {
		return nil
	}
	match := failoverResetTimePattern.FindStringSubmatch(trimmed[loc[1]:])
	if match == nil {
		return nil
	}

	datePart, sep, timePart, offsetPart, zonePart := match[1], match[2], match[3], match[4], match[5]
	var parsed time.Time
	var err error

	switch {
	case offsetPart != "" && zonePart != "":
		// 数字偏移与缩写同时存在时，Go 以偏移为准并接受未知缩写。
		parsed, err = time.Parse("2006-01-02 15:04:05 -0700 MST",
			datePart+" "+timePart+" "+normalizeUTCOffset(offsetPart)+" "+zonePart)
	case offsetPart != "" && sep == "T":
		parsed, err = time.Parse(time.RFC3339, datePart+"T"+timePart+normalizeRFC3339Offset(offsetPart))
	case offsetPart != "":
		parsed, err = time.Parse("2006-01-02 15:04:05 -0700",
			datePart+" "+timePart+" "+normalizeUTCOffset(offsetPart))
	case zonePart != "":
		// 仅有缩写时区（如单独的 CST）存在歧义，不做猜测。
		return nil
	case sep == "T":
		// 无时区信息：按服务器本地时区解释。
		parsed, err = time.ParseInLocation("2006-01-02T15:04:05", datePart+"T"+timePart, now.Location())
	default:
		parsed, err = time.ParseInLocation("2006-01-02 15:04:05", datePart+" "+timePart, now.Location())
	}
	if err != nil || !isValidFailoverAutoRecoverAt(parsed, now) {
		return nil
	}
	return &parsed
}

// normalizeUTCOffset 把 +08:00 / +0800 / Z 统一成 +0800 形式。
func normalizeUTCOffset(offset string) string {
	if offset == "Z" {
		return "+0000"
	}
	if len(offset) == 6 && offset[3] == ':' {
		return offset[:3] + offset[4:]
	}
	return offset
}

// normalizeRFC3339Offset 把 +0800 / Z 转成 RFC3339 需要的 +08:00 形式。
func normalizeRFC3339Offset(offset string) string {
	if offset == "Z" {
		return "Z"
	}
	if len(offset) == 5 {
		return offset[:3] + ":" + offset[3:]
	}
	return offset
}

func isValidFailoverAutoRecoverAt(recoverAt, now time.Time) bool {
	return recoverAt.After(now) && !recoverAt.After(now.Add(failoverAutoRecoverMaxAge))
}

// ensureFailoverAutoRecoverTag 保证 error_message 带上自动恢复标记。
func ensureFailoverAutoRecoverTag(msg string, recoverAt time.Time) string {
	if strings.Contains(msg, FailoverAutoRecoverMessageTag) {
		return msg
	}
	return fmt.Sprintf("%s %s", strings.TrimSpace(msg), formatFailoverAutoRecoverTag(recoverAt))
}
