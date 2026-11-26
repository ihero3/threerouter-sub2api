package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// authProjectionRequiredFields 是鉴权链路（api_key_auth 中间件 + 其调用的风控服务）
// 直接读取、因而必须出现在 GetByKeyForAuth 用户投影里的字段。
//
// 为什么要一张表而不是逐个写断言：
//  1. ent 的 q.Select(...) 是白名单投影，未列出的字段一律返回零值且不报错；
//  2. 这些零值会被 api_key_auth_cache_impl 原样写进认证快照，再一路传到判定点，
//     表现为「规则看起来存在、实际永不生效」——没有日志、没有错误码；
//  3. 同一处投影已经因为裁剪漏字段出过两次问题（risk_level、register_ip/created_at），
//     所以把口径集中在一张表里，新增依赖时只加一行，遗漏的可能性更低。
//
// 新增字段时同步这里，并确认 ent 常量确实存在于 ent/user/user.go。
var authProjectionRequiredFields = []struct {
	entField  string // 必须出现在 GetByKeyForAuth 的 q.Select(...) 中
	consumer  string // 谁在消费它（写清失效后果，便于后人判断能不能删）
	failureIf string // 缺失时的静默失效表现
}{
	{
		entField:  "user.FieldRiskLevel",
		consumer:  "middleware.RiskAccessBlocked（待复核账号冻结闸门）",
		failureIf: "RiskLevel 恒为空，待复核账号可正常调用网关，缓存重建的请求绕过冻结",
	},
	{
		entField:  "user.FieldRegisterIP",
		consumer:  "service.IPConsistencyService.CheckNewUserFirstUseIP（注册后 3 分钟严格规则）",
		failureIf: "RegisterIP 恒为空串，函数在 registerIP == \"\" 处静默放行，严格规则从未生效",
	},
	{
		entField:  "user.FieldCreatedAt",
		consumer:  "service.IPConsistencyService.CheckNewUserFirstUseIP（窗口判定）",
		failureIf: "CreatedAt 恒为零值，函数在 createdAt.IsZero() 处静默放行，严格规则从未生效",
	},
	{
		// 注意：这是缓存命中后由 api_key_service 层补的，不在投影里也拿得到，
		// 但鉴权闸门在 DB 回源路径会用原始值做余额判断，故一并锁住。
		entField:  "user.FieldBalance",
		consumer:  "api_key_auth 中间件余额前置校验",
		failureIf: "Balance 恒为 0，回源请求被误判为余额不足",
	},
}

// getByKeyForAuthBody 抽出 GetByKeyForAuth 的函数体文本，供静态断言使用。
func getByKeyForAuthBody(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("api_key_repo.go")
	require.NoError(t, err, "无法读取 api_key_repo.go")

	const marker = "func (r *apiKeyRepository) GetByKeyForAuth"
	start := strings.Index(string(src), marker)
	require.GreaterOrEqual(t, start, 0, "找不到 GetByKeyForAuth 定义，投影守卫需要同步更新")

	body := string(src[start:])
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	return body
}

// TestGetByKeyForAuthProjectionIncludesRequiredFields 锁死鉴权 DB 回源的用户字段投影。
//
// GetByKeyForAuth 用 q.Select(...) 显式列出所需 user 字段以裁剪数据传输，ent 对未
// Select 的字段一律返回零值。任何一条依赖这些字段的闸门都会因此静默失效——不会报错，
// 只能靠哨兵锁死。
//
// 背景（2026-11-26 线上实锤）：用户 6687 注册后 142 秒换 IP 调用成功并扣费，
// 排查发现 register_ip / created_at 都不在投影里，「注册后 3 分钟 IP 严格规则」
// 自上线起从未生效过。
func TestGetByKeyForAuthProjectionIncludesRequiredFields(t *testing.T) {
	body := getByKeyForAuthBody(t)

	for _, f := range authProjectionRequiredFields {
		require.Contains(t, body, f.entField,
			"GetByKeyForAuth 的用户投影必须包含 %s（消费方：%s）；缺失后果：%s",
			f.entField, f.consumer, f.failureIf)
	}
}

// TestGetByKeyForAuthProjectionFieldsExistInEnt 防止清单里写错常量名（拼写错误会让
// 上一条断言永远为假阴性：Contains 找不到这串，但也没人在意它到底存不存在）。
func TestGetByKeyForAuthProjectionFieldsExistInEnt(t *testing.T) {
	src, err := os.ReadFile("../../ent/user/user.go")
	require.NoError(t, err, "无法读取 ent/user/user.go")

	entSrc := string(src)
	for _, f := range authProjectionRequiredFields {
		// entField 形如 "user.FieldBalance"，在 ent/user/user.go 里对应 `FieldBalance = "..."`。
		constName := strings.TrimPrefix(f.entField, "user.")
		require.Contains(t, entSrc, constName+" =",
			"清单里的 %s 在 ent/user/user.go 中不存在，请检查常量名", f.entField)
	}
}
