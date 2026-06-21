import { Holding, PortfolioRecord, SyncStatus, UserInfo } from "./types"

const BASE = ""

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(BASE + url, {
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    ...options,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  return res.json()
}

export async function fetchHoldings(): Promise<Holding[]> {
  return request<Holding[]>("/api/holdings")
}

export async function createHolding(h: Omit<Holding, "id">): Promise<Holding> {
  return request<Holding>("/api/holdings", {
    method: "POST",
    body: JSON.stringify(h),
  })
}

export async function updateHolding(id: string, updates: Partial<Holding>): Promise<Holding> {
  return request<Holding>(`/api/holdings/${id}`, {
    method: "PATCH",
    body: JSON.stringify(updates),
  })
}

export async function deleteHolding(id: string): Promise<void> {
  await request<{ success: boolean }>(`/api/holdings/${id}`, {
    method: "DELETE",
  })
}

export async function sellHolding(
  id: string,
  shares: number,
  price: number,
  fee: number,
  value: number
): Promise<{ soldHolding: Holding; availableFunds: string }> {
  return request<{ soldHolding: Holding; availableFunds: string }>(`/api/holdings/${id}/sell`, {
    method: "POST",
    body: JSON.stringify({ shares, price, fee, value }),
  })
}

export async function fetchRecords(): Promise<PortfolioRecord[]> {
  return request<PortfolioRecord[]>("/api/records")
}

export async function createRecord(): Promise<PortfolioRecord> {
  return request<PortfolioRecord>("/api/records", {
    method: "POST",
  })
}

export async function deleteRecord(id: string): Promise<void> {
  await request<{ success: boolean }>(`/api/records/${id}`, {
    method: "DELETE",
  })
}

export async function fetchSettings(): Promise<Record<string, string>> {
  return request<Record<string, string>>("/api/settings")
}

export async function updateSetting(
  key: string,
  value: string
): Promise<{ key: string; value: string }> {
  return request<{ key: string; value: string }>(`/api/settings/${key}`, {
    method: "PUT",
    body: JSON.stringify({ value }),
  })
}

export async function updateSettings(
  settings: Record<string, string>
): Promise<Record<string, string>> {
  return request<Record<string, string>>("/api/settings", {
    method: "PUT",
    body: JSON.stringify(settings),
  })
}

export async function fetchAvailableFunds(): Promise<{ value: string }> {
  return request<{ value: string }>("/api/funds")
}

export async function updateAvailableFunds(value: string): Promise<{ key: string; value: string }> {
  return request<{ key: string; value: string }>("/api/funds", {
    method: "PUT",
    body: JSON.stringify({ value }),
  })
}

export async function fetchSyncStatus(): Promise<SyncStatus> {
  return request<SyncStatus>("/api/sync/status")
}

export async function fetchPrice(symbol: string): Promise<{
  symbol: string
  name: string
  price: number
  originalPrice: number
  currency: string
  originalCurrency: string
  unit: string
}> {
  return request(`/api/price/${encodeURIComponent(symbol)}`)
}

export async function fetchExchangeRate(pair: string): Promise<{ rate: number }> {
  return request(`/api/exchange/${encodeURIComponent(pair)}`)
}

export async function triggerSync(): Promise<SyncStatus> {
  return request<SyncStatus>("/api/sync/trigger", { method: "POST" })
}

export async function testTelegramConnection(
  botToken: string,
  chatID: string
): Promise<{ success: boolean; botName?: string; error?: string }> {
  return request<{ success: boolean; botName?: string; error?: string }>(
    "/api/telegram/test",
    {
      method: "POST",
      body: JSON.stringify({ botToken, chatID, type: "connection" }),
    }
  )
}

export async function testTelegramMessage(
  botToken: string,
  chatID: string,
  type: "price" | "drift" | "summary"
): Promise<{ success: boolean; error?: string }> {
  return request<{ success: boolean; error?: string }>(
    "/api/telegram/test",
    {
      method: "POST",
      body: JSON.stringify({ botToken, chatID, type }),
    }
  )
}

export async function fetchSetupStatus(): Promise<{ configured: boolean }> {
  return request<{ configured: boolean }>("/api/setup/status")
}

export async function submitSetup(config: {
  databaseType: string
  databaseDsn: string
  username: string
  password: string
}): Promise<{ success: boolean }> {
  return request<{ success: boolean }>("/api/setup/complete", {
    method: "POST",
    body: JSON.stringify(config),
  })
}

export async function login(username: string, password: string): Promise<{ user: UserInfo }> {
  return request<{ user: UserInfo }>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  })
}

export async function logout(): Promise<void> {
  await request<{ success: boolean }>("/api/auth/logout", { method: "POST" })
}

export async function fetchMe(): Promise<UserInfo> {
  return request<UserInfo>("/api/auth/me")
}

export async function register(username: string, password: string, role: string): Promise<UserInfo> {
  return request<UserInfo>("/api/users", {
    method: "POST",
    body: JSON.stringify({ username, password, role }),
  })
}

export async function listUsers(): Promise<UserInfo[]> {
  return request<UserInfo[]>("/api/users")
}

export async function deleteUser(id: string): Promise<void> {
  await request<{ success: boolean }>(`/api/users/${id}`, { method: "DELETE" })
}

export interface OIDCConfig {
  enabled: boolean
  issuer: string
  clientID: string
  clientSecret: string
  redirectURL: string
}

export async function fetchOIDCConfig(): Promise<OIDCConfig> {
  return request<OIDCConfig>("/api/oidc/config")
}

export async function updateOIDCConfig(config: OIDCConfig): Promise<OIDCConfig> {
  return request<OIDCConfig>("/api/oidc/config", {
    method: "PUT",
    body: JSON.stringify(config),
  })
}

export interface WebAuthnConfig {
  enabled: boolean
  rpid: string
  rpOrigins: string[]
}

export async function fetchWebAuthnConfig(): Promise<WebAuthnConfig> {
  return request<WebAuthnConfig>("/api/oidc/webauthn-config")
}

export async function updateWebAuthnConfig(config: WebAuthnConfig): Promise<WebAuthnConfig> {
  return request<WebAuthnConfig>("/api/oidc/webauthn-config", {
    method: "PUT",
    body: JSON.stringify(config),
  })
}

export interface WebAuthnCredentialInfo {
  id: string
  name: string
  createdAt: number
  lastUsedAt: number
}

export async function webAuthnRegisterStart(name: string): Promise<any> {
  return request("/api/webauthn/register/start", {
    method: "POST",
    body: JSON.stringify({ name }),
  })
}

export async function webAuthnRegisterFinish(data: any): Promise<{ success: string }> {
  return request("/api/webauthn/register/finish", {
    method: "POST",
    body: JSON.stringify(data),
  })
}

export async function webAuthnLoginStart(): Promise<any> {
  return request("/api/webauthn/login/start", { method: "POST" })
}

export async function webAuthnLoginFinish(data: any): Promise<{ user: UserInfo }> {
  return request<{ user: UserInfo }>("/api/webauthn/login/finish", {
    method: "POST",
    body: JSON.stringify(data),
  })
}

export async function webAuthnListCredentials(): Promise<WebAuthnCredentialInfo[]> {
  return request<WebAuthnCredentialInfo[]>("/api/webauthn/credentials")
}

export async function webAuthnDeleteCredential(id: string): Promise<void> {
  await request<{ success: boolean }>(`/api/webauthn/credentials/${id}`, { method: "DELETE" })
}
