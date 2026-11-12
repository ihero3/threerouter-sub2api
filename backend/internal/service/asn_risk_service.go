package service

import (
	"context"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/oschwald/maxminddb-golang"
)

// DefaultRegistrationDatacenterASNs 是后台设置项 registration_datacenter_asns 的默认值。
//
// 这是一份「提高对抗成本的弱信号」清单，不是强判定：命中机房 / 云厂商 IP 时只把注册点击
// 验证码难度提升到 high，绝不拦截注册，也不自动把账号标记为 suspect / abnormal，以免误伤
// 真实 VPS 用户。命中与否只影响一次人机验证的解题成本，不影响账号状态或任何权益。
//
// 每项可以是纯 ASN 数字（也接受 AS16509 写法），也可以是组织名片段（大小写不敏感的子串
// 匹配）。组织名片段比 ASN 更稳健：同一云厂商常有多个 ASN，且组织名比号段更少变动。
const DefaultRegistrationDatacenterASNs = "16509,14618,15169,396982,8075,14061,20473,63949,24940,16276,31898,45102,37963,132203,13335,9009,49981," +
	"amazon,aws,google,microsoft,azure,digitalocean,vultr,linode,akamai,hetzner,ovh,oracle,alibaba,aliyun,tencent,cloudflare,choopa,leaseweb,contabo,scaleway,psychz,m247"

// ASNRecord 是 GeoLite2-ASN 数据库的最小映射结构，仅取 ASN 号与组织名。
type ASNRecord struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

// ASNRiskService 基于离线 GeoLite2-ASN.mmdb 判定客户端 IP 是否属于常见机房 / 云厂商。
//
// 设计要点（与 GeoIPService 同范式，见 docs/合规方案.md 4.1.2 降级策略）：
//   - 数据库未配置 / 文件缺失 / 加载失败时只记日志，服务照常启动，reader 为 nil；
//   - 全链路 fail-open：reader 为 nil、IP 非法、私有 / 回环 / 链路本地、查询报错、
//     清单为空一律返回「未命中」，绝不因 ASN 能力异常阻断注册；
//   - 命中结果仅用于把点击验证码难度提升为 high，不做任何拦截或风险标记。
type ASNRiskService struct {
	reader *maxminddb.Reader
	// mu 保护 reader 的并发访问与关闭。
	mu sync.RWMutex
	// lookupFn 是可替换的查询实现，仅用于 bare 层测试不依赖真实 mmdb 文件模拟命中；
	// 生产恒为 nil，走 reader 查询。IP 合法性 / 私有地址校验在调用它之前统一完成。
	lookupFn func(ip string) (ASNRecord, bool)
}

// NewASNRiskService 根据合规配置创建 ASN 风险服务。
//
// 无论加载成功与否都返回非 nil 的 *ASNRiskService，保证 wire 注入始终成功：
// ASNDatabasePath 为空或打开失败时 reader 为 nil，行为退化为全量 fail-open。
// 加载失败仅记录日志，不返回错误（避免离线库缺失导致整个服务无法启动）。
func NewASNRiskService(cfg *config.Config) *ASNRiskService {
	svc := &ASNRiskService{}
	if cfg == nil {
		return svc
	}
	path := strings.TrimSpace(cfg.Compliance.GeoIP.ASNDatabasePath)
	if path == "" {
		slog.Info("asn_risk.init_skipped", "reason", "empty asn_database_path")
		return svc
	}
	reader, err := maxminddb.Open(path)
	if err != nil {
		slog.Warn("asn_risk.open_failed", "path", path, "error", err.Error())
		return svc
	}
	svc.reader = reader
	slog.Info("asn_risk.ready", "path", path, "database_type", reader.Metadata.DatabaseType)
	return svc
}

// Available 报告 ASN 离线库是否已就绪（可执行真实查询）。
func (a *ASNRiskService) Available() bool {
	if a == nil {
		return false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.reader != nil
}

// Close 释放底层数据库句柄。多次调用安全。
func (a *ASNRiskService) Close() error {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.reader == nil {
		return nil
	}
	err := a.reader.Close()
	a.reader = nil
	return err
}

// Lookup 解析给定 IP 的 ASN 号与组织名。
//
// 返回 (记录, true) 表示查到一条可用于判定的公网记录；其余情况一律返回 (零值, false)：
// svc 为 nil、reader 不可用、IP 无法解析、私有 / 回环 / 链路本地地址、查询报错。
// 私有 / 回环 / 链路本地 IP 不参与机房判定（与 GeoIPService 口径一致）。
func (a *ASNRiskService) Lookup(clientIP string) (ASNRecord, bool) {
	if a == nil {
		return ASNRecord{}, false
	}
	ipAddr := net.ParseIP(strings.TrimSpace(clientIP))
	if ipAddr == nil {
		return ASNRecord{}, false
	}
	if ipAddr.IsPrivate() || ipAddr.IsLoopback() || ipAddr.IsLinkLocalUnicast() || ipAddr.IsLinkLocalMulticast() {
		return ASNRecord{}, false
	}

	// 测试注入点：优先使用可替换查询实现，生产此分支恒不进入。
	if a.lookupFn != nil {
		return a.lookupFn(ipAddr.String())
	}

	a.mu.RLock()
	reader := a.reader
	a.mu.RUnlock()
	if reader == nil {
		return ASNRecord{}, false
	}
	var record ASNRecord
	if err := reader.Lookup(ipAddr, &record); err != nil {
		return ASNRecord{}, false
	}
	return record, true
}

// IsDatacenterIP 报告给定客户端 IP 是否命中机房 / 云厂商清单。
//
// 任何异常（服务不可用、非法 / 私有 IP、查询失败、清单为空）一律返回 false（fail-open）。
// ctx 保留在签名中以便调用方传递链路信息；离线库查询本身不使用它。
func (a *ASNRiskService) IsDatacenterIP(ctx context.Context, clientIP, configuredList string) bool {
	_ = ctx
	if a == nil {
		return false
	}
	record, ok := a.Lookup(clientIP)
	if !ok {
		return false
	}
	return MatchDatacenterASN(record.AutonomousSystemNumber, record.AutonomousSystemOrganization, configuredList)
}

// MatchDatacenterASN 判定一条 ASN 记录是否命中机房 / 云厂商清单（纯函数，便于不依赖 mmdb 测试）。
//
// 清单按逗号分隔，每项去空格、大小写不敏感：
//   - 纯数字项（接受可选 AS 前缀，如 16509 / AS16509）→ 与 ASN 号精确匹配；
//   - 其余项 → 作为组织名片段做包含匹配（大小写不敏感）。
//
// 清单为空（无有效项）→ 返回 false：管理员清空清单即关闭判定，设置读取失败也按未命中处理，
// 符合全链路 fail-open；不使用内置默认清单做隐式兜底，避免「读不到配置反而触发升级」。
func MatchDatacenterASN(asn uint, org, configuredList string) bool {
	org = strings.ToLower(strings.TrimSpace(org))
	asnText := strconv.FormatUint(uint64(asn), 10)
	for _, raw := range strings.Split(configuredList, ",") {
		item := strings.ToLower(strings.TrimSpace(raw))
		if item == "" {
			continue
		}
		// 兼容 "AS16509" / "as16509" 写法，仅当后缀为纯数字时才剥离前缀，
		// 避免误改以 "as" 开头的组织名片段。
		if len(item) > 2 && strings.HasPrefix(item, "as") && isAllDigits(item[2:]) {
			item = item[2:]
		}
		if item == "" {
			continue
		}
		if isAllDigits(item) {
			if item == asnText {
				return true
			}
			continue
		}
		if org != "" && strings.Contains(org, item) {
			return true
		}
	}
	return false
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// GetRegistrationDatacenterASNs 读取后台机房 / 云厂商 ASN 清单（逗号分隔原始串）。
//
// 返回 (value, found)：found=false 表示设置项缺失或 DB 暂不可用，调用方应按「未命中」
// fail-open 处理，不使用内置默认清单做隐式兜底。新装系统的默认清单由
// InitializeDefaultSettings 写入 SettingKeyRegistrationDatacenterASNs（见 setting_parse.go）。
func (s *SettingService) GetRegistrationDatacenterASNs(ctx context.Context) (string, bool) {
	if s == nil {
		return "", false
	}
	return s.getCachedSettingValue(ctx, SettingKeyRegistrationDatacenterASNs)
}
