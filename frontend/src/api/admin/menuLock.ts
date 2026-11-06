import { apiClient } from "../client";

export interface AdminMenuUnlockRequest {
  menu: string;
  password: string;
}

export interface AdminMenuUnlockResponse {
  unlocked: boolean;
}

/** 校验后台敏感菜单访问密码（默认密码由后端持有）。 */
export async function unlockAdminMenu(
  password: string,
  menu: string,
): Promise<AdminMenuUnlockResponse> {
  const payload: AdminMenuUnlockRequest = { password, menu };
  const { data } = await apiClient.post<AdminMenuUnlockResponse>(
    "/admin/menu-unlock",
    payload,
  );
  return data;
}
