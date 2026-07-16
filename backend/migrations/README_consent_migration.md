# 同意记录迁移指南

## 问题

新版本的程序增加了同意记录强制校验功能。已发布服务器上的现有用户没有同意记录，会导致无法使用 API。

## 解决方案

在部署新版本之前，需要先为所有现有用户创建默认的同意记录。

## 迁移步骤

### 1. 备份数据库（重要！）

在执行迁移之前，请务必备份数据库：

```bash
# PostgreSQL 备份命令
pg_dump -h <host> -p <port> -U <user> -d <dbname> > backup_$(date +%Y%m%d_%H%M%S).sql
```

### 2. 执行迁移脚本

#### 方法一：使用 Go 迁移程序（推荐）

1. 将代码部署到服务器
2. 进入后端目录：`cd /path/to/backend`
3. 执行迁移：

```bash
go run cmd/migrate_default_consents/main.go "host=localhost port=5432 dbname=sub2api user=sub2api password=sub2api sslmode=disable"
```

替换上面的连接字符串为实际的数据库配置。

#### 方法二：使用 SQL 脚本

直接执行 SQL 迁移文件：

```bash
psql -h <host> -p <port> -U <user> -d <dbname> -f migrations/162_default_consents_for_existing_users.sql
```

### 3. 验证迁移结果

执行以下 SQL 查询验证迁移是否成功：

```sql
-- 检查每个用户的同意记录数量
SELECT 
    u.id,
    u.email,
    COUNT(uc.id) as consent_count
FROM users u
LEFT JOIN user_consents uc ON u.id = uc.user_id
GROUP BY u.id, u.email
ORDER BY consent_count;
```

每个用户应该有 6 条同意记录（如果之前没有的话）。

### 4. 部署新版本

迁移完成后，可以安全地部署包含同意记录校验的新版本。

## 迁移内容

为每个现有用户创建以下默认同意记录（已授予状态）：

| 同意类型 | 状态 | 说明 |
|---------|------|------|
| terms_of_service | ✅ 已授予 | 服务条款 |
| gdpr_data_processing | ✅ 已授予 | GDPR 数据处理协议 |
| detailed_logging | ✅ 已授予 | 详细日志记录 |
| cross_border_transfer | ✅ 已授予 | 跨境数据传输 |
| marketing | ✅ 已授予 | 营销信息 |
| model_training | ✅ 已授予 | 模型训练数据 |

## 注意事项

1. **备份优先**：执行迁移前务必备份数据库
2. **幂等性**：迁移脚本可以安全地多次执行，不会重复创建记录
3. **不影响现有数据**：迁移只会为缺少同意记录的用户创建记录，不会影响已有的同意记录
4. **审计追踪**：迁移创建的同意记录会记录 `source` 为 `migration_default`，便于审计

## 回滚方案

如果需要回滚，可以删除迁移创建的同意记录：

```sql
DELETE FROM user_consents WHERE source = 'migration_default';
```

**注意**：回滚后，现有用户将无法使用 API，因为新版本会强制检查同意记录。
