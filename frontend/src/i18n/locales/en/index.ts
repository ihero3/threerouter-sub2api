import adminAccessKeys from './adminAccessKeys'
import mcp from './mcp'
import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import admin from './admin'
import misc from './misc'

export default {
  adminAccessKeys,
  mcp,
  ...landing,
  ...common,
  ...dashboard,
  ...channelMonitorV2,
  ...batchImage,
  // admin 命名空间由 ./admin/* 与 common.ts 中的 admin 段（如 admin.blog）合并而成。
  // 不能直接用 `admin,` 覆盖，否则 common 里的 admin.* 键会丢失。
  admin: { ...common.admin, ...admin },
  ...misc,
}
