import type {
  PublicKeyCredentialCreationOptionsJSON,
  PublicKeyCredentialRequestOptionsJSON,
  RegistrationResponseJSON,
  AuthenticationResponseJSON,
} from "@simplewebauthn/browser"
import {
  Account,
  Dividend,
  Holding,
  HoldingWithAccount,
  MergedHolding,
  MarketSourceConfig,
  Portfolio,
  PortfolioRecord,
  PortfolioSummary,
  SourceTestComplete,
  SourceTestResult,
  SyncStatus,
  TestSourcesResult,
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
  if (res.status === 204) return undefined as T
  return res.json()
}

export async function fetchHoldings(pid: string, currency?: string): Promise<MergedHolding[]> {
  const params = new URLSearchParams()
  params.set("merge", "true")
  if (currency) params.set("currency", currency)
  return request<MergedHolding[]>(`/api/portfolios/${pid}/holdings?${params.toString()}`)
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

export async function deleteLot(pid: string, hid: string, lid: string): Promise<Holding> {
  return request<Holding>(`/api/portfolios/${pid}/holdings/${hid}/lots/${lid}`, {
    method: "DELETE",
  })
}

export async function updateLot(
  pid: string,
  hid: string,
  lid: string,
  data: Partial<import("./types").HoldingLot>
): Promise<Holding> {
  return request<Holding>(`/api/portfolios/${pid}/holdings/${hid}/lots/${lid}`, {
    method: "PATCH",
    body: JSON.stringify(data),
  })
}

export async function sellHolding(
  pid: string,
  id: string,
  shares: string,
  price: string,
  fee: string,
  value: string,
  date?: string
): Promise<{ soldHolding: Holding; availableFunds: string }> {
  return request<{ soldHolding: Holding; availableFunds: string }>(
    `/api/portfolios/${pid}/holdings/${id}/sell`,
    {
      method: "POST",
      body: JSON.stringify({ shares, price, fee, value, date }),
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
): Promise<{ currency: string; amount: string }[]> {
  return request<{ currency: string; amount: string }[]>(`/api/portfolios/${pid}/funds`)
}

export async function transferInFunds(
  pid: string,
  currency: string,
  amount: string,
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
  amount: string,
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
  amount: string,
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
  fromAmount: string,
  toAmount: string,
  exchangeRate: string
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

export async function triggerSync(pid: string): Promise<SyncStatus> {
  return request<SyncStatus>(`/api/portfolios/${pid}/sync/trigger`, { method: "POST" })
}

export async function fetchPrice(
  symbol: string,
  market?: string
): Promise<{
  symbol: string
  name: string
  price: number
  originalPrice: number
  currency: string
  originalCurrency: string
  unit: string
}> {
  const params = market ? `?market=${market}` : ""
  return request(`/api/price/${encodeURIComponent(symbol)}${params}`)
}

export async function fetchExchangeRate(pair: string): Promise<{ rate: string }> {
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
  type: "price" | "drift" | "summary",
  portfolioId: string
): Promise<{ success: boolean; error?: string }> {
  return request<{ success: boolean; error?: string }>("/api/telegram/test", {
    method: "POST",
    body: JSON.stringify({ botToken, chatID, type, portfolioId }),
  })
}

export async function testBarkConnection(
  deviceKey: string,
  serverURL: string
): Promise<{ success: boolean; error?: string }> {
  return request<{ success: boolean; error?: string }>("/api/bark/test", {
    method: "POST",
    body: JSON.stringify({ deviceKey, serverURL, type: "connection" }),
  })
}

export async function testBarkMessage(
  deviceKey: string,
  serverURL: string,
  type: "price" | "drift" | "summary",
  portfolioId: string
): Promise<{ success: boolean; error?: string }> {
  return request<{ success: boolean; error?: string }>("/api/bark/test", {
    method: "POST",
    body: JSON.stringify({ deviceKey, serverURL, type, portfolioId }),
  })
}

export async function fetchSetupStatus(): Promise<{ configured: boolean }> {
  return request<{ configured: boolean }>("/api/setup/status")
}

export async function submitSetup(config: {
  databaseType: string
  databaseHost: string
  databasePort: string
  databaseName: string
  databaseUsername: string
  databasePassword: string
  databaseSslMode: string
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

// Account API
export async function fetchAccounts(): Promise<Account[]> {
  return request<Account[]>("/api/accounts")
}

export async function createAccount(data: {
  name: string
  description?: string
  broker?: string
}): Promise<Account> {
  return request<Account>("/api/accounts", {
    method: "POST",
    body: JSON.stringify(data),
  })
}

export async function updateAccount(
  id: string,
  updates: { name?: string; description?: string; broker?: string }
): Promise<Account> {
  return request<Account>(`/api/accounts/${id}`, {
    method: "PATCH",
    body: JSON.stringify(updates),
  })
}

export async function deleteAccount(id: string): Promise<void> {
  await request<{ success: boolean }>(`/api/accounts/${id}`, {
    method: "DELETE",
  })
}

export async function fetchAccountViewHoldings(
  currency?: string,
  accountId?: string
): Promise<HoldingWithAccount[]> {
  const params = new URLSearchParams()
  if (currency) params.set("currency", currency)
  if (accountId) params.set("account_id", accountId)
  const qs = params.toString()
  return request<HoldingWithAccount[]>(`/api/accounts/all-holdings${qs ? "?" + qs : ""}`)
}

export async function fetchMarketSources(): Promise<MarketSourceConfig> {
  return request<MarketSourceConfig>("/api/settings/market-sources")
}

export async function updateMarketSources(
  config: Record<string, string[]>
): Promise<{ status: string }> {
  return request<{ status: string }>("/api/settings/market-sources", {
    method: "PUT",
    body: JSON.stringify(config),
  })
}

export async function testMarketSources(
  config: Record<string, string[]>
): Promise<TestSourcesResult> {
  return request<TestSourcesResult>("/api/settings/market-sources/test", {
    method: "POST",
    body: JSON.stringify(config),
  })
}

export interface SourceTestStreamOptions {
  onResult: (key: string, source: string, market: string, result: SourceTestResult) => void
  onComplete: (summary: SourceTestComplete) => void
  onError: (error: Error) => void
}

export async function testMarketSourcesStream(
  config: Record<string, string[]>,
  options: SourceTestStreamOptions
): Promise<void> {
  const res = await fetch(BASE + "/api/settings/market-sources/test", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(config),
  })

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }

  const reader = res.body!.getReader()
  const decoder = new TextDecoder()
  let buffer = ""

  try {
    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split("\n")
      buffer = lines.pop() || ""

      let eventType = ""
      for (const line of lines) {
        if (line.startsWith("event: ")) {
          eventType = line.slice(7).trim()
        } else if (line.startsWith("data: ")) {
          const data = line.slice(6)
          try {
            const parsed = JSON.parse(data)
            if (eventType === "source-test-result") {
              options.onResult(parsed.key, parsed.source, parsed.market, parsed.result)
            } else if (eventType === "source-test-complete") {
              options.onComplete(parsed)
            }
          } catch {
            // skip malformed data
          }
        }
      }
    }
  } catch (err) {
    options.onError(err instanceof Error ? err : new Error(String(err)))
  }
}

// Dividend API
export async function recordDividend(
  pid: string,
  data: {
    holdingId: string
    grossAmount: string
    taxAmount: string
    type: "cash" | "reinvest"
    paymentDate: string
    reinvestmentPrice: string
    note?: string
  }
): Promise<Dividend> {
  return request<Dividend>(`/api/portfolios/${pid}/dividends`, {
    method: "POST",
    body: JSON.stringify(data),
  })
}

export async function fetchDividends(pid: string, holdingId?: string): Promise<Dividend[]> {
  const params = holdingId ? `?holdingId=${holdingId}` : ""
  return request<Dividend[]>(`/api/portfolios/${pid}/dividends${params}`)
}

export async function deleteDividend(pid: string, id: string): Promise<void> {
  await request<void>(`/api/portfolios/${pid}/dividends/${id}`, {
    method: "DELETE",
  })
}

export async function updateDividend(
  pid: string,
  id: string,
  data: {
    grossAmount: string
    taxAmount: string
    type: "cash" | "reinvest"
    paymentDate: string
    reinvestmentPrice: string
    note?: string
  }
): Promise<Dividend> {
  return request<Dividend>(`/api/portfolios/${pid}/dividends/${id}`, {
    method: "PATCH",
    body: JSON.stringify(data),
  })
}
