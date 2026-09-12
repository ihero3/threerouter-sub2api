package service

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	VideoBillingResolution480P  = "480p"
	VideoBillingResolution720P  = "720p"
	VideoBillingResolution1080P = "1080p"
	// VideoBillingResolution768P / 2K / 4K 是厂商专有档位：MiniMax H3 用 768P / 2K，
	// 部分模型支持 4K。管理员可在 video_model_prices 里直接按这些键配单价。
	VideoBillingResolution768P = "768p"
	VideoBillingResolution2K   = "2k"
	VideoBillingResolution4K   = "4k"
)

// 视频生成按秒计费，各厂商支持的时长差异极大：xAI Grok Imagine 是 1-15 秒，
// 而 Seedance / 可灵 / Wan / Sora 等模型支持 30 秒甚至更长。因此计费时长**不设业务上限**，
// 一律以"上游回传真实时长 > 用户请求时长 > 默认 8 秒"的优先级取实际值，
// 否则长视频会被少收（此前统一钳到 15 秒，30 秒视频只收 15 秒的钱）。
const (
	VideoBillingMinDurationSeconds     = 1
	VideoBillingDefaultDurationSeconds = 8
	// VideoBillingSanityMaxDurationSeconds 不是业务时长上限，只用于拦截上游脏数据
	// （例如把毫秒当秒返回、或返回 999999 这类异常值）。超过该阈值视为无效数据，
	// 继续向下一优先级回退（用户请求时长 → 默认时长），而不是钳到该值。
	VideoBillingSanityMaxDurationSeconds = 3600
)

// NormalizeVideoBillingDurationSecondsOrDefault 归一化计费用视频时长：
// 未指定（<=0）或上游返回明显异常的值时按默认 8 秒计，其余按实际值计费，不设上限。
func NormalizeVideoBillingDurationSecondsOrDefault(durationSeconds int) int {
	if usable := sanitizeVideoBillingDuration(durationSeconds); usable > 0 {
		return usable
	}
	return VideoBillingDefaultDurationSeconds
}

// NormalizeVideoBillingDurationSeconds 按优先级确定计费时长：
// 上游回传的真实时长 > 用户请求时长 > 默认 8 秒。
//
// 真实时长优先，防止用户传 1 秒却实际生成更长视频的套利；上游不回传时长时回退到
// 用户请求值，避免一律按默认 8 秒多收。两者都缺失才用默认值。
// 三个来源都只做有效性校验，不做业务时长钳制。
func NormalizeVideoBillingDurationSeconds(actualSeconds, requestedSeconds int) int {
	if usable := sanitizeVideoBillingDuration(actualSeconds); usable > 0 {
		return usable
	}
	if usable := sanitizeVideoBillingDuration(requestedSeconds); usable > 0 {
		return usable
	}
	return VideoBillingDefaultDurationSeconds
}

// sanitizeVideoBillingDuration 校验单个时长值是否可用于计费，返回 0 表示不可用。
// 只做两件事：下界（<=0 视为缺失）与脏数据上界（超过 1 小时，典型是上游把毫秒当秒返回）。
func sanitizeVideoBillingDuration(durationSeconds int) int {
	if durationSeconds < VideoBillingMinDurationSeconds {
		return 0
	}
	if durationSeconds > VideoBillingSanityMaxDurationSeconds {
		return 0
	}
	return durationSeconds
}

// LookupVideoBillingResolution 归一化分辨率并报告是否为已知档位。
// 配置解析路径必须用它而不是 OrDefault：把无法识别的档位（如 "4k"、拼错的
// "1080i"）静默折算成 480p，会让管理员配的高分辨率单价被挂到低分辨率档上。
func LookupVideoBillingResolution(resolution string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "480", "480p", "sd":
		return VideoBillingResolution480P, true
	case "720", "720p", "hd":
		return VideoBillingResolution720P, true
	case "1080", "1080p", "full_hd", "full-hd", "fhd":
		return VideoBillingResolution1080P, true
	default:
		return "", false
	}
}

// LookupVideoBillingResolutionAny 归一化计费档位并保留厂商专有档位。
// MiniMax H3 用 768P / 2K，这些档位在其他厂商不存在。管理员可以在
// video_model_prices 里直接按 "768p" / "2k" 配置单价；这里原样保留，
// 而不是把它们静默折算成 480p。
//
// 除档位名外，还接受用户直接传的长宽尺寸（"1920x1080"、"1920*1080"、
// "1080:1920"）与纯数字（"1080"），按短边映射到最近档位。
func LookupVideoBillingResolutionAny(resolution string) (string, bool) {
	if normalized, ok := LookupVideoBillingResolution(resolution); ok {
		return normalized, true
	}
	normalized := strings.ToLower(strings.TrimSpace(resolution))
	switch normalized {
	case "768", "768p":
		return VideoBillingResolution768P, true
	case "2k", "1440p", "2kp", "1440":
		return VideoBillingResolution2K, true
	case "4k", "2160p", "4kp", "2160":
		return VideoBillingResolution4K, true
	}
	return ParseVideoBillingResolution(normalized)
}

// videoBillingDimensionPattern 匹配用户传入的长宽尺寸，如 1920x1080 / 1920*1080 /
// 1080:1920 / 1920×1080。分隔符两侧允许空格。
var videoBillingDimensionPattern = regexp.MustCompile(`^(\d{2,5})\s*[xX*×:：·]\s*(\d{2,5})$`)

// ParseVideoBillingResolution 解析分辨率表达式为计费档位：
//  1. 长宽尺寸（1920x1080 等）取**短边**映射 —— 竖屏 1080x1920 与横屏 1920x1080
//     都算 1080p，符合视频行业按纵向像素命名的惯例；
//  2. 纯数字（"1080"）按纵向像素映射。
//
// 无法解析时返回 false，由调用方决定兜底策略。
func ParseVideoBillingResolution(resolution string) (string, bool) {
	raw := strings.ToLower(strings.TrimSpace(resolution))
	if raw == "" {
		return "", false
	}
	if match := videoBillingDimensionPattern.FindStringSubmatch(raw); match != nil {
		width, errW := strconv.Atoi(match[1])
		height, errH := strconv.Atoi(match[2])
		if errW == nil && errH == nil && width > 0 && height > 0 {
			shortSide := width
			if height < shortSide {
				shortSide = height
			}
			return VideoBillingResolutionFromPixels(shortSide), true
		}
	}
	if pixels, err := strconv.Atoi(raw); err == nil && pixels > 0 {
		return VideoBillingResolutionFromPixels(pixels), true
	}
	return "", false
}

// VideoBillingResolutionFromPixels 按短边像素数映射到计费档位（向上取最近档）。
// 超过 1440 统一归到 4k；价格表里没有 4k 时会沿降档链回退，见
// VideoBillingResolutionFallbacks。
func VideoBillingResolutionFromPixels(shortSidePixels int) string {
	switch {
	case shortSidePixels <= 480:
		return VideoBillingResolution480P
	case shortSidePixels <= 720:
		return VideoBillingResolution720P
	case shortSidePixels <= 768:
		return VideoBillingResolution768P
	case shortSidePixels <= 1080:
		return VideoBillingResolution1080P
	case shortSidePixels <= 1440:
		return VideoBillingResolution2K
	default:
		return VideoBillingResolution4K
	}
}

// videoBillingResolutionOrder 计费档位由高到低的顺序。
var videoBillingResolutionOrder = []string{
	VideoBillingResolution4K,
	VideoBillingResolution2K,
	VideoBillingResolution1080P,
	VideoBillingResolution768P,
	VideoBillingResolution720P,
	VideoBillingResolution480P,
}

// VideoBillingResolutionFallbacks 返回分辨率的降档查找序列（含自身，由高到低）。
// 价格表没配高档位时逐级向低档回退，避免直接掉到 480p 或默认价造成静默错价。
// 无法识别的档位退化为仅查自身；空输入退化为只查 480p。
func VideoBillingResolutionFallbacks(resolution string) []string {
	tier, ok := LookupVideoBillingResolutionAny(resolution)
	if !ok {
		if strings.TrimSpace(resolution) == "" {
			return []string{VideoBillingResolution480P}
		}
		return []string{strings.ToLower(strings.TrimSpace(resolution))}
	}
	start := -1
	for i, candidate := range videoBillingResolutionOrder {
		if candidate == tier {
			start = i
			break
		}
	}
	if start < 0 {
		return []string{tier}
	}
	out := make([]string, 0, len(videoBillingResolutionOrder)-start)
	return append(out, videoBillingResolutionOrder[start:]...)
}

// NormalizeVideoBillingResolutionAnyOrDefault 与上者相同，但未知档位兜底到 480p。
func NormalizeVideoBillingResolutionAnyOrDefault(resolution string) string {
	if normalized, ok := LookupVideoBillingResolutionAny(resolution); ok {
		return normalized
	}
	return VideoBillingResolution480P
}

// NormalizeVideoBillingResolutionOrDefault 用于运行时计费：上游回传的分辨率
// 缺失或无法识别时按最低档兜底，保证请求仍可计费。
func NormalizeVideoBillingResolutionOrDefault(resolution string) string {
	if normalized, ok := LookupVideoBillingResolution(resolution); ok {
		return normalized
	}
	return VideoBillingResolution480P
}
