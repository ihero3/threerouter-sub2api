package service

import "strings"

// ModelCapability 表示模型对外提供的能力（模态）。
// 统一入口据此把请求分派到对应链路，让客户端只关心 model 而不用挑端点。
type ModelCapability string

const (
	ModelCapabilityText  ModelCapability = "text"
	ModelCapabilityImage ModelCapability = "image"
	ModelCapabilityVideo ModelCapability = "video"
	ModelCapabilityAudio ModelCapability = "audio"
)

// DispatchModelCapability 按模型名判定模态。
//
// 判定顺序与 MediaKindFromModel 保持一致：**图片优先于视频**。
// 否则 "qwen-image" 这类名字里的 video 相关子串会造成误判
// （MediaKindFromModel 明确注释过这个坑）。
//
// 无法识别的模型一律按 text 处理：文本是默认链路，未知模型走
// chat 转发既不会误伤，也能让上游给出权威的错误信息。
func DispatchModelCapability(model string) ModelCapability {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return ModelCapabilityText
	}
	switch {
	case IsKnownImageVendorModel(m):
		return ModelCapabilityImage
	case IsKnownVideoVendorModel(m):
		return ModelCapabilityVideo
	case IsKnownAudioVendorModel(m):
		return ModelCapabilityAudio
	default:
		return ModelCapabilityText
	}
}
