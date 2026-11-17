package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGetByKeyForAuthProjectionIncludesRiskLevelGuard 锁死鉴权 DB 回源的用户字段投影。
//
// GetByKeyForAuth 用 q.Select(...) 显式列出所需 user 字段以裁剪数据传输，ent 对未
// Select 的字段一律返回零值。网关的「待复核账号冻结」依赖 User.RiskLevel，一旦有人
// 做字段裁剪时把它删掉，DB 回源路径的 RiskLevel 就会恒为空并被写进认证快照，
// 缓存未命中/重建的请求将静默绕过闸门 —— 这类失效不会有任何报错，只能靠哨兵锁死。
//
// 这里读源码做静态断言而非起真库：仓库该层没有 sqlmock 基建，引入真库/新依赖
// 对一条「字段别被删」的守卫来说得不偿失。
func TestGetByKeyForAuthProjectionIncludesRiskLevelGuard(t *testing.T) {
	src, err := os.ReadFile("api_key_repo.go")
	require.NoError(t, err, "无法读取 api_key_repo.go")

	const marker = "func (r *apiKeyRepository) GetByKeyForAuth"
	start := strings.Index(string(src), marker)
	require.GreaterOrEqual(t, start, 0, "找不到 GetByKeyForAuth 定义，投影守卫需要同步更新")

	body := string(src[start:])
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	require.Contains(t, body, "user.FieldRiskLevel",
		"GetByKeyForAuth 的用户投影必须包含 risk_level，否则鉴权闸门在 DB 回源路径失效")
}
