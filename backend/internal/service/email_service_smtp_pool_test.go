package service

import (
	"bufio"
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 上方「SMTP 设置」的单账号与 smtp_accounts 列表一起参与轮换。
//
// 修复前 SendEmail 只要列表非空就整组丢弃单账号配置，最早配置的账号加了
// 多账号后再也不发信。本文件锁死合并/去重语义，并做一次端到端投递验证。
// 刻意不加 //go:build unit（与项目基线一致），基建自带、不依赖 unit 层 helper。
// ---------------------------------------------------------------------------

func TestMergeSingleSMTPAccount_AppendsSingleAccount(t *testing.T) {
	listed := SMTPAccountSetting{ID: "list-1", Host: "smtp.example.com", Username: "listed@example.com", Enabled: true, DailyLimit: 300}
	single := &SMTPAccountSetting{Host: "smtp.example.com", Username: "primary@example.com", Enabled: true, DailyLimit: 500}

	pool := mergeSingleSMTPAccount([]SMTPAccountSetting{listed}, []SMTPAccountSetting{listed}, single)

	require.Len(t, pool, 2)
	require.Equal(t, "list-1", pool[0].ID, "列表账号的相对顺序不变")
	require.Equal(t, "primary@example.com", pool[1].Username)
	require.Equal(t, smtpAccountDefaultDailyLimit, pool[1].DailyLimit)
	require.True(t, pool[1].Enabled)
}

func TestMergeSingleSMTPAccount_DeduplicatesByHostAndUsername(t *testing.T) {
	listed := SMTPAccountSetting{ID: "list-1", Host: "SMTP.Gmail.com", Username: "SAME@GMAIL.COM", Enabled: true, DailyLimit: 300}
	single := &SMTPAccountSetting{Host: "smtp.gmail.com", Username: "same@gmail.com", Enabled: true, DailyLimit: 500}

	pool := mergeSingleSMTPAccount([]SMTPAccountSetting{listed}, []SMTPAccountSetting{listed}, single)

	require.Len(t, pool, 1, "同一 host+username 不得重复入池，否则日额度翻倍、同一邮箱被计数两次")
	require.Equal(t, "list-1", pool[0].ID, "重复时以带 ID 与可配额度的列表账号为准")
}

func TestMergeSingleSMTPAccount_DisabledListAccountSuppressesSingle(t *testing.T) {
	disabled := SMTPAccountSetting{ID: "list-1", Host: "smtp.gmail.com", Username: "same@gmail.com", Enabled: false}
	single := &SMTPAccountSetting{Host: "smtp.gmail.com", Username: "same@gmail.com", Enabled: true}

	pool := mergeSingleSMTPAccount(nil, []SMTPAccountSetting{disabled}, single)

	require.Empty(t, pool, "管理员在列表里显式禁用的账号，不应因上方还留着单账号配置而被重新启用")
}

func TestMergeSingleSMTPAccount_NilSingleKeepsListUnchanged(t *testing.T) {
	listed := SMTPAccountSetting{ID: "list-1", Host: "smtp.example.com", Username: "listed@example.com", Enabled: true}

	pool := mergeSingleSMTPAccount([]SMTPAccountSetting{listed}, []SMTPAccountSetting{listed}, nil)

	require.Len(t, pool, 1)
	require.Equal(t, "list-1", pool[0].ID)
}

// smtpPoolSettingRepoStub 只提供 EmailService 需要的两个读取入口。
type smtpPoolSettingRepoStub struct {
	accountsJSON string
	values       map[string]string
}

func (s *smtpPoolSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (s *smtpPoolSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if key == SettingKeySMTPAccounts {
		return s.accountsJSON, nil
	}
	return "", ErrSettingNotFound
}
func (s *smtpPoolSettingRepoStub) Set(context.Context, string, string) error { return nil }
func (s *smtpPoolSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}
func (s *smtpPoolSettingRepoStub) SetMultiple(context.Context, map[string]string) error { return nil }
func (s *smtpPoolSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}
func (s *smtpPoolSettingRepoStub) Delete(context.Context, string) error { return nil }

func newSMTPPoolSettingRepo(t *testing.T, listed []SMTPAccountSetting, singleValues map[string]string) *smtpPoolSettingRepoStub {
	t.Helper()
	raw, err := marshalSMTPAccounts(listed)
	require.NoError(t, err)
	return &smtpPoolSettingRepoStub{accountsJSON: raw, values: singleValues}
}

func TestSingleSMTPAccount_ReadsPrimarySettings(t *testing.T) {
	repo := newSMTPPoolSettingRepo(t, nil, map[string]string{
		SettingKeySMTPHost:     "smtp.gmail.com",
		SettingKeySMTPPort:     "587",
		SettingKeySMTPUsername: "ihero.cn@gmail.com",
		SettingKeySMTPPassword: "app-password",
		SettingKeySMTPFrom:     "ihero.cn@gmail.com",
		SettingKeySMTPFromName: "Three Router",
		SettingKeySMTPUseTLS:   "true",
	})
	svc := NewEmailService(repo, nil)

	acc := svc.singleSMTPAccount(context.Background())

	require.NotNil(t, acc)
	require.Equal(t, "smtp.gmail.com", acc.Host)
	require.Equal(t, 587, acc.Port)
	require.Equal(t, "ihero.cn@gmail.com", acc.Username)
	require.Equal(t, "app-password", acc.Password)
	require.Equal(t, "Three Router", acc.FromName)
	require.True(t, acc.UseTLS)
	require.True(t, acc.Enabled)
	require.Equal(t, smtpAccountDefaultDailyLimit, acc.DailyLimit)
	require.Empty(t, acc.ID, "单账号不造 ID，让 cacheKey 按 host+username 派生稳定标识")
}

func TestSingleSMTPAccount_NotConfiguredReturnsNil(t *testing.T) {
	svc := NewEmailService(newSMTPPoolSettingRepo(t, nil, map[string]string{}), nil)
	require.Nil(t, svc.singleSMTPAccount(context.Background()), "未配置单账号时轮换池只含列表账号，保持旧行为")
}

// smtpPoolFakeServer 是不 advertise STARTTLS 的最小明文 SMTP 服务器。
// host 用 127.0.0.1，net/smtp 的 PlainAuth 允许本地明文连接。
type smtpPoolFakeServer struct {
	listener net.Listener
	mu       sync.Mutex
	mailFrom []string
	done     chan struct{}
}

func startSMTPPoolFakeServer(t *testing.T) (*smtpPoolFakeServer, int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &smtpPoolFakeServer{listener: listener, done: make(chan struct{})}
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-srv.done:
		case <-time.After(2 * time.Second):
		}
	})
	go srv.serve()
	return srv, listener.Addr().(*net.TCPAddr).Port
}

func (s *smtpPoolFakeServer) serve() {
	defer close(s.done)
	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	reply := func(line string) bool {
		if _, err := writer.WriteString(line + "\r\n"); err != nil {
			return false
		}
		return writer.Flush() == nil
	}
	if !reply("220 localhost ESMTP ready") {
		return
	}
	inData := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if inData {
			if line == "." {
				inData = false
				if !reply("250 2.0.0 queued") {
					return
				}
			}
			continue
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			if !reply("250-localhost\r\n250 AUTH PLAIN") {
				return
			}
		case strings.HasPrefix(upper, "AUTH "):
			if !reply("235 2.7.0 authentication successful") {
				return
			}
		case strings.HasPrefix(upper, "MAIL FROM"):
			s.mu.Lock()
			s.mailFrom = append(s.mailFrom, line)
			s.mu.Unlock()
			if !reply("250 2.1.0 ok") {
				return
			}
		case strings.HasPrefix(upper, "RCPT TO"):
			if !reply("250 2.1.5 ok") {
				return
			}
		case upper == "DATA":
			inData = true
			if !reply("354 end with <CRLF>.<CRLF>") {
				return
			}
		case upper == "QUIT":
			_ = reply("221 2.0.0 bye")
			return
		default:
			if !reply("250 2.0.0 ok") {
				return
			}
		}
	}
}

func (s *smtpPoolFakeServer) senders() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.mailFrom...)
}

// TestSendEmail_FailsOverToPrimarySMTPAccount 端到端锁死「单账号真的进了轮换池」：
// 列表里的账号指向一个必然连接失败的端口，只有上方单账号指向可用的本地 SMTP。
// 修复前 SendEmail 根本不会尝试单账号，本用例必然失败。
func TestSendEmail_FailsOverToPrimarySMTPAccount(t *testing.T) {
	server, port := startSMTPPoolFakeServer(t)
	repo := newSMTPPoolSettingRepo(t, []SMTPAccountSetting{
		{
			ID:         "list-1",
			Host:       "127.0.0.1",
			Port:       1, // 无服务，连接立即失败
			Username:   "listed@example.com",
			Password:   "pw",
			From:       "listed@example.com",
			Enabled:    true,
			DailyLimit: 500,
		},
	}, map[string]string{
		SettingKeySMTPHost:     "127.0.0.1",
		SettingKeySMTPPort:     strconv.Itoa(port),
		SettingKeySMTPUsername: "primary@example.com",
		SettingKeySMTPPassword: "pw",
		SettingKeySMTPFrom:     "primary@example.com",
		SettingKeySMTPFromName: "Three Router",
		SettingKeySMTPUseTLS:   "false",
	})
	svc := NewEmailService(repo, nil)

	err := svc.SendEmail(context.Background(), "user@example.com", "subject", "<p>body</p>")

	require.NoError(t, err)
	senders := server.senders()
	require.Len(t, senders, 1, "应通过上方单账号投递一次")
	require.Contains(t, senders[0], "primary@example.com")
}
