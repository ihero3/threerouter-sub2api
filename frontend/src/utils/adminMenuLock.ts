/**
 * 后台敏感菜单的二次访问密码状态（按菜单粒度，仅当前浏览器会话有效）。
 * 密码本身由后端校验，前端只记录哪些菜单已在本次会话中解锁。
 */

export const ADMIN_MENU_UNLOCK_EVENT = 'admin-menu-unlock-required'

export const ADMIN_MENU_LOCKED_PATHS = [
  '/admin/accounts',
  '/admin/channels',
  '/admin/groups',
  '/admin/settings',
] as const

const STORAGE_KEY = 'admin_menu_unlocked_paths'

export interface AdminMenuUnlockDetail {
  /** 解锁成功后要跳转的完整路径（含 query） */
  path: string
  /** 命中的受保护菜单前缀 */
  menuPath: string
}

/** 返回路径命中的受保护菜单前缀，未命中返回 null。 */
export function matchAdminMenuLockedPath(path: string): string | null {
  const matched = (ADMIN_MENU_LOCKED_PATHS as readonly string[]).find(
    (prefix) => path === prefix || path.startsWith(prefix + '/')
  )
  return matched ?? null
}

function readUnlockedMenus(): string[] {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter(
      (item): item is string =>
        typeof item === 'string' && (ADMIN_MENU_LOCKED_PATHS as readonly string[]).includes(item)
    )
  } catch {
    return []
  }
}

export function isAdminMenuPathUnlocked(menuPath: string): boolean {
  return readUnlockedMenus().includes(menuPath)
}

export function markAdminMenuPathUnlocked(menuPath: string): void {
  const unlocked = readUnlockedMenus()
  if (unlocked.includes(menuPath)) return
  unlocked.push(menuPath)
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(unlocked))
  } catch {
    // sessionStorage 不可用时仅本次导航放行，忽略持久化失败
  }
}

export function clearAdminMenuUnlockState(): void {
  try {
    sessionStorage.removeItem(STORAGE_KEY)
  } catch {
    // ignore
  }
}

/** 路由守卫拦截时通知全局弹窗输入访问密码。 */
export function requestAdminMenuUnlock(path: string, menuPath: string): void {
  try {
    window.dispatchEvent(
      new CustomEvent<AdminMenuUnlockDetail>(ADMIN_MENU_UNLOCK_EVENT, {
        detail: { path, menuPath },
      })
    )
  } catch {
    // ignore event failures
  }
}
