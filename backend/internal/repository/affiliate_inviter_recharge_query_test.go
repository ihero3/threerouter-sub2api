package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestInviterRechargeQueriesMatchRebateBaseContract 锁死「邀请人累计实充」的 SQL 口径：
// 只统计已履约完成（COMPLETED + paid_at 非空）的余额/订阅订单，余额单剔除赠送额度，
// 与单笔返利基数 paymentOrderAmountWithoutBonus 保持同口径；累计返利只统计 accrue。
func TestInviterRechargeQueriesMatchRebateBaseContract(t *testing.T) {
	source, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	content := strings.Join(strings.Fields(string(source)), " ")

	require.Contains(t, content, "status = 'COMPLETED'")
	require.Contains(t, content, "paid_at IS NOT NULL")
	require.Contains(t, content, "order_type IN ('balance', 'subscription')")
	require.Contains(t, content, "amount - bonus_amount")

	require.Contains(t, content, "FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'accrue'")
}
