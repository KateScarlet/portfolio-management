import type {
  PublicKeyCredentialCreationOptionsJSON,
  PublicKeyCredentialRequestOptionsJSON,
  RegistrationResponseJSON,
  AuthenticationResponseJSON,
} from "@simplewebauthn/browser"
import {
  Holding,
  Portfolio,
  PortfolioRecord,
  PortfolioSummary,
  SyncStatus,
  UserInfo,
} from "./types"

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

export async function fetchHoldings(pid: string, currency?: string): Promise<Holding[]> {
  const params = currency ? `?currency=${currency}` : ""
  return request<Holding[]>(`/api/portfolios/${pid}/holdings${params}`)
}

export async function createHolding(pid: string, h: Omit<Holding, "id">): Promise<Holding> {
  return request<Holding>(`/api/portfolios/${pid}/holdings`, {
    method: "POST",
    body: JSON.stringify(h),
  })
}

export async function updateHolding(
  pid: string,
  id: string,
  updates: Partial<Holding>
): Promise<Holding> {
  return request<Holding>(`/api/portfolios/${pid}/holdings/${id}`, {
    method: "PATCH",
    body: JSON.stringify(updates),
  })
}

export async function deleteHolding(pid: string, id: string): Promise<void> {
  await request<{ success: boolean }>(`/api/portfolios/${pid}/holdings/${id}`, {
    method: "DELETE",
  })
}

export async function sellHolding(
  pid: string,
  id: string,
  shares: number,
  price: number,
  fee: number,
  value: number
): Promise<{ soldHolding: Holding; availableFunds: string }> {
  return request<{ soldHolding: Holding; availableFunds: string }>(
    `/api/portfolios/${pid}/holdings/${id}/sell`,
    {
      method: "POST",
      body: JSON.stringify({ shares, price, fee, value }),
    }
  )
}

export async function fetchRecords(pid: string): Promise<PortfolioRecord[]> {
  return request<PortfolioRecord[]>(`/api/portfolios/${pid}/records`)
}

export async function createRecord(pid: string, currency?: string): Promise<PortfolioRecord> {
  const params = currency ? `?currency=${currency}` : ""
  return request<PortfolioRecord>(`/api/portfolios/${pid}/records${params}`, {
    method: "POST",
  })
}

export async function deleteRecord(pid: string, id: string): Promise<void> {
  await request<{ success: boolean }>(`/api/portfolios/${pid}/records/${id}`, {
    method: "DELETE",
  })
}

export async function fetchSettings(pid: string): Promise<Record<string, string>> {
  return request<Record<string, string>>(`/api/portfolios/${pid}/settings`)
}

export async function updateSetting(
  pid: string,
  key: string,
  value: string
): Promise<{ key: string; value: string }> {
  return request<{ key: string; value: string }>(`/api/portfolios/${pid}/settings/${key}`, {
    method: "PUT",
    body: JSON.stringify({ value }),
  })
}

export async function updateSettings(
  pid: string,
  settings: Record<string, string>
): Promise<Record<string, string>> {
  return request<Record<string, string>>(`/api/portfolios/${pid}/settings`, {
    method: "PUT",
    body: JSON.stringify(settings),
  })
}

export async function fetchAvailableFunds(
  pid: string
): Promise<{ currency: string; amount: number }[]> {
  return request<{ currency: string; amount: number }[]>(`/api/portfolios/${pid}/funds`)
}

export async function transferInFunds(
  pid: string,
  currency: string,
  amount: number,
  note: string
): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/portfolios/${pid}/funds/transfer-in`, {
    method: "POST",
    body: JSON.stringify({ currency, amount, note }),
  })
}

export async function transferOutFunds(
  pid: string,
  currency: string,
  amount: number,
  note: string
): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/portfolios/${pid}/funds/transfer-out`, {
    method: "POST",
    body: JSON.stringify({ currency, amount, note }),
  })
}

export async function transferBetweenFunds(
  pid: string,
  currency: string,
  amount: number,
  targetPortfolioId: string,
  note: string
): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/portfolios/${pid}/funds/transfer`, {
    method: "POST",
    body: JSON.stringify({ currency, amount, targetPortfolioId, note }),
  })
}

export async function convertCurrency(
  pid: string,
  fromCurrency: string,
  toCurrency: string,
  fromAmount: number,
  toAmount: number,
  exchangeRate: number
): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/portfolios/${pid}/funds/convert`, {
    method: "POST",
    body: JSON.stringify({ fromCurrency, toCurrency, fromAmount, toAmount, exchangeRate }),
  })
}

export async function fetchFundTransactions(
  pid: string,
  type?: string
): Promise<import("./types").FundTransaction[]> {
  const params = type ? `?type=${type}` : ""
  return request<import("./types").FundTransaction[]>(
    `/api/portfolios/${pid}/fund-transactions${params}`
  )
}

export async function fetchSyncStatus(pid: string): Promise<SyncStatus> {
  return request<SyncStatus>(`/api/portfolios/${pid}/sync/status`)
}

export async function triggerSync(pid: string): Promise<SyncStatus> {
  return request<SyncStatus>(`/api/portfolios/${pid}/sync/trigger`, { method: "POST" })
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

export async function fetchPortfolios(): Promise<Portfolio[]> {
  return request<Portfolio[]>("/api/portfolios")
}

export async function createPortfolio(name: string, description?: string): Promise<Portfolio> {
  return request<Portfolio>("/api/portfolios", {
    method: "POST",
    body: JSON.stringify({ name, description }),
  })
}

export async function updatePortfolio(
  id: string,
  updates: { name?: string; description?: string }
): Promise<Portfolio> {
  return request<Portfolio>(`/api/portfolios/${id}`, {
    method: "PATCH",
    body: JSON.stringify(updates),
  })
}

export async function deletePortfolio(id: string): Promise<void> {
  await request<{ success: boolean }>(`/api/portfolios/${id}`, {
    method: "DELETE",
  })
}

export async function fetchSummary(): Promise<PortfolioSummary> {
  return request<PortfolioSummary>("/api/summary")
}

export async function testTelegramConnection(
  botToken: string,
  chatID: string
): Promise<{ success: boolean; botName?: string; error?: string }> {
  return request<{ success: boolean; botName?: string; error?: string }>("/api/telegram/test", {
    method: "POST",
    body: JSON.stringify({ botToken, chatID, type: "connection" }),
  })
}

export async function testTelegramMessage(
  botToken: string,
  chatID: string,
  type: "price" | "drift" | "summary"
): Promise<{ success: boolean; error?: string }> {
  return request<{ success: boolean; error?: string }>("/api/telegram/test", {
    method: "POST",
    body: JSON.stringify({ botToken, chatID, type }),
  })
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

export async function register(
  username: string,
  password: string,
  role: string
): Promise<UserInfo> {
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

export async function webAuthnRegisterStart(
  name: string
): Promise<PublicKeyCredentialCreationOptionsJSON> {
  return request("/api/webauthn/register/start", {
    method: "POST",
    body: JSON.stringify({ name }),
  })
}

export async function webAuthnRegisterFinish(
  data: RegistrationResponseJSON
): Promise<{ success: string }> {
  return request("/api/webauthn/register/finish", {
    method: "POST",
    body: JSON.stringify(data),
  })
}

export async function webAuthnLoginStart(): Promise<PublicKeyCredentialRequestOptionsJSON> {
  return request("/api/webauthn/login/start", { method: "POST" })
}

export async function webAuthnLoginFinish(
  data: AuthenticationResponseJSON
): Promise<{ user: UserInfo }> {
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
