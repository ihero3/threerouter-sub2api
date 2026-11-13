package middleware

import "strings"

// authErrorDefaultUserID 是登录/注册类错误（被国家/地区封禁或 IP 黑名单拦截、
// 或登录失败无法确定用户）的兜底归属用户。统一记为 admin(id=1)，使其能在
// 「使用记录 → 错误请求」（按当前登录用户过滤）中被观测到，防止登录坍塌毫不知情。
const authErrorDefaultUserID = 1

// platformForPath 按路径推断平台标识：面板 API 记为 "panel"，其余（模型网关等）留空。
// 仅用于封禁落库的展示归类，不影响任何鉴权或路由逻辑。
func platformForPath(path string) string {
	if strings.HasPrefix(path, "/api/") {
		return "panel"
	}
	return ""
}
