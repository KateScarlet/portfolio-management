import { useState, useEffect, useCallback, useRef } from "react"
import Decimal from "decimal.js"
import { ChartPie } from "lucide-react"
import { usePortfolio } from "./usePortfolio"
import { useExchangeRates } from "./useExchangeRates"
import { useSSE } from "./hooks/useSSE"
import {
  Account,
  Settings,
  SyncStatus,
  UserInfo,
  Portfolio,
  PortfolioSummary,
  DEFAULT_SETTINGS,
  ColorScheme,
  Holding,
  AvailableFund,
  FundTransaction,
} from "./types"
import type { SSEEvent, SyncCompletedData, SyncFailedData, PriceUpdatedData } from "./types/events"
import * as api from "./api"
import { toDecimal } from "./utils"
import Dashboard from "./components/Dashboard"
import HoldingsManager from "./components/HoldingsManager"
import RebalancePanel from "./components/RebalancePanel"
import HistoryPanel from "./components/HistoryPanel"
import SettingsPanel from "./components/SettingsPanel"
import SetupWizard from "./components/SetupWizard"
import LoginPage from "./components/LoginPage"
import UserManager from "./components/UserManager"
import PortfolioSelector from "./components/PortfolioSelector"
import PortfolioManager from "./components/PortfolioManager"
import SummaryDashboard from "./components/SummaryDashboard"
import AccountView from "./components/AccountView"
import AccountSelector from "./components/AccountSelector"
import AccountManager from "./components/AccountManager"

const STORAGE_KEY = "selectedPortfolioId"
const ACCOUNT_STORAGE_KEY = "selectedAccountId"

function getStoredPortfolioId(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
}

function setStoredPortfolioId(id: string | null) {
  try {
    if (id) {
      localStorage.setItem(STORAGE_KEY, id)
    } else {
      localStorage.removeItem(STORAGE_KEY)
    }
  } catch {
    // Ignore localStorage errors
  }
}

function getStoredAccountId(): string | null {
  try {
    return localStorage.getItem(ACCOUNT_STORAGE_KEY)
  } catch {
    return null
  }
}

function setStoredAccountId(id: string | null) {
  try {
    if (id) {
      localStorage.setItem(ACCOUNT_STORAGE_KEY, id)
    } else {
      localStorage.removeItem(ACCOUNT_STORAGE_KEY)
    }
  } catch {
    // Ignore localStorage errors
  }
}

export default function App() {
  const [setupMode, setSetupMode] = useState<boolean | null>(null)
  const [user, setUser] = useState<UserInfo | null>(null)
  const [authChecked, setAuthChecked] = useState(false)
  const [portfolios, setPortfolios] = useState<Portfolio[]>([])
  const [currentPortfolio, setCurrentPortfolioState] = useState<Portfolio | null>(null)
  const [showPortfolioManager, setShowPortfolioManager] = useState(false)
  const [viewMode, setViewMode] = useState<"portfolio" | "account">(
    () => (localStorage.getItem("viewMode") as "portfolio" | "account") || "portfolio"
  )
  const [accounts, setAccounts] = useState<Account[]>([])
  const [currentAccount, setCurrentAccount] = useState<Account | null>(null)
  const [showAccountManager, setShowAccountManager] = useState(false)
  const accountsLoadedRef = useRef(false)

  useEffect(() => {
    localStorage.setItem("viewMode", viewMode)
  }, [viewMode])

  useEffect(() => {
    if (currentAccount) {
      setStoredAccountId(currentAccount.id)
    } else if (accountsLoadedRef.current) {
      setStoredAccountId(null)
    }
  }, [currentAccount])

  // Restore selected account from localStorage when accounts load
  useEffect(() => {
    if (accounts.length > 0) {
      accountsLoadedRef.current = true
    }
  }, [accounts])

  const setCurrentPortfolio = useCallback(
    (portfolio: Portfolio | null | ((prev: Portfolio | null) => Portfolio | null)) => {
      setCurrentPortfolioState((prev) => {
        const next = typeof portfolio === "function" ? portfolio(prev) : portfolio
        setStoredPortfolioId(next?.id || null)
        return next
      })
    },
    []
  )
  const [showSummary, setShowSummary] = useState(false)
  const [summary, setSummary] = useState<PortfolioSummary | null>(null)
  const [summaryLoading, setSummaryLoading] = useState(false)
  const [summaryError, setSummaryError] = useState("")
  const [settings, setSettings] = useState<Settings>(DEFAULT_SETTINGS)
  const [syncStatus, setSyncStatus] = useState<SyncStatus | null>(null)
  const [availableFunds, setAvailableFunds] = useState<AvailableFund[]>([])
  const [fundTransactions, setFundTransactions] = useState<FundTransaction[]>([])
  const exchangeRates = useExchangeRates(availableFunds, settings.displayCurrency)

  const {
    holdings,
    setHoldings,
    assets,
    history,
    loading,
    addHolding,
    updateHolding,
    removeHolding,
    saveRecord,
    deleteRecord,
  } = usePortfolio(currentPortfolio?.id || null, settings.displayCurrency, exchangeRates)

  const totalFundsDisplay = availableFunds.reduce((sum, f) => {
    const rate = exchangeRates[f.currency]
    return rate ? sum.plus(toDecimal(f.amount).times(rate)) : sum
  }, new Decimal(0))

  const handleSSEEvent = useCallback(
    (event: SSEEvent) => {
      if (event.portfolioId !== currentPortfolio?.id) return

      switch (event.type) {
        case "sync.started":
          setSyncStatus((prev) => (prev ? { ...prev, syncing: true } : null))
          break

        case "sync.completed": {
          const data = event.data as SyncCompletedData
          setSyncStatus({
            syncing: false,
            lastSyncAt: data.lastSyncAt,
            lastSyncErr: "",
          })
          break
        }

        case "sync.failed": {
          const data = event.data as SyncFailedData
          setSyncStatus((prev) =>
            prev ? { ...prev, syncing: false, lastSyncErr: data.error } : null
          )
          break
        }

        case "price.updated": {
          const data = event.data as PriceUpdatedData
          setHoldings((prev) =>
            prev.map((h) => {
              const updated = data.holdings.find((u) => u.symbol === h.symbol)
              return updated
                ? { ...h, price: String(updated.price), value: String(updated.value) }
                : h
            })
          )
          break
        }
      }
    },
    [currentPortfolio?.id, setHoldings]
  )

  useSSE({
    onEvent: handleSSEEvent,
    enabled: !!user && !!currentPortfolio,
  })

  useEffect(() => {
    api
      .fetchSetupStatus()
      .then((s) => setSetupMode(!s.configured))
      .catch(() => setSetupMode(false))
  }, [])

  useEffect(() => {
    if (setupMode === false) {
      api
        .fetchMe()
        .then((u) => setUser(u))
        .catch(() => setUser(null))
        .finally(() => setAuthChecked(true))
    }
  }, [setupMode])

  const loadPortfolios = useCallback(async () => {
    try {
      let ps = await api.fetchPortfolios()
      if (ps.length === 0) {
        const created = await api.createPortfolio("默认组合")
        ps = [created]
      }
      setPortfolios(ps)
      setCurrentPortfolio((prev) => {
        if (prev) {
          const existing = ps.find((p) => p.id === prev.id)
          if (existing) return existing
        }
        const storedId = getStoredPortfolioId()
        if (storedId) {
          const stored = ps.find((p) => p.id === storedId)
          if (stored) return stored
        }
        return ps[0]
      })
    } catch (e) {
      console.error("Failed to load portfolios", e)
    }
  }, [setCurrentPortfolio])

  const applyAccounts = useCallback((list: Account[]) => {
    setAccounts(list)
    setCurrentAccount((prev) => {
      if (prev) {
        return list.find((a) => a.id === prev.id) || null
      }
      const storedId = getStoredAccountId()
      if (storedId) {
        return list.find((a) => a.id === storedId) || null
      }
      return null
    })
  }, [])

  const loadAccounts = useCallback(async () => {
    try {
      const list = await api.fetchAccounts()
      applyAccounts(list)
    } catch (e) {
      console.error("Failed to load accounts", e)
    }
  }, [applyAccounts])

  useEffect(() => {
    if (!user) return
    let cancelled = false
    const fetchPortfolios = async () => {
      try {
        let ps = await api.fetchPortfolios()
        if (ps.length === 0) {
          const created = await api.createPortfolio("默认组合")
          ps = [created]
        }
        if (!cancelled) {
          setPortfolios(ps)
          setCurrentPortfolio((prev) => {
            if (prev) {
              const existing = ps.find((p) => p.id === prev.id)
              if (existing) return existing
            }
            const storedId = getStoredPortfolioId()
            if (storedId) {
              const stored = ps.find((p) => p.id === storedId)
              if (stored) return stored
            }
            return ps[0]
          })
        }
      } catch (e) {
        console.error("Failed to load portfolios", e)
      }
    }
    fetchPortfolios()
    return () => {
      cancelled = true
    }
  }, [user, setCurrentPortfolio])

  useEffect(() => {
    if (!user) return
    let cancelled = false
    const fetchAccounts = async () => {
      try {
        const list = await api.fetchAccounts()
        if (!cancelled) applyAccounts(list)
      } catch (e) {
        console.error("Failed to load accounts", e)
      }
    }
    fetchAccounts()
    return () => {
      cancelled = true
    }
  }, [user, applyAccounts])

  useEffect(() => {
    if (!currentPortfolio) return
    api
      .fetchSettings(currentPortfolio.id)
      .then((s) => {
        setSettings({
          driftThreshold:
            s.driftThreshold != null ? Number(s.driftThreshold) : DEFAULT_SETTINGS.driftThreshold,
          syncInterval:
            s.syncInterval != null ? Number(s.syncInterval) : DEFAULT_SETTINGS.syncInterval,
          colorScheme: (s.colorScheme as ColorScheme) || DEFAULT_SETTINGS.colorScheme,
          targetStocks:
            s.targetStocks != null ? Number(s.targetStocks) : DEFAULT_SETTINGS.targetStocks,
          targetBonds: s.targetBonds != null ? Number(s.targetBonds) : DEFAULT_SETTINGS.targetBonds,
          targetCash: s.targetCash != null ? Number(s.targetCash) : DEFAULT_SETTINGS.targetCash,
          targetCommodities:
            s.targetCommodities != null
              ? Number(s.targetCommodities)
              : s.targetGold != null
                ? Number(s.targetGold)
                : DEFAULT_SETTINGS.targetCommodities,
          telegramBotToken: s.telegramBotToken || DEFAULT_SETTINGS.telegramBotToken,
          telegramChatID: s.telegramChatID || DEFAULT_SETTINGS.telegramChatID,
          telegramEnabled: s.telegramEnabled === "true",
          telegramPriceAlert: s.telegramPriceAlert !== "false",
          telegramDriftAlert: s.telegramDriftAlert !== "false",
          telegramSummary: s.telegramSummary !== "false",
          telegramPriceThreshold:
            s.telegramPriceThreshold != null
              ? Number(s.telegramPriceThreshold)
              : DEFAULT_SETTINGS.telegramPriceThreshold,
          telegramSummaryInterval:
            s.telegramSummaryInterval || DEFAULT_SETTINGS.telegramSummaryInterval,
          displayCurrency: s.displayCurrency || DEFAULT_SETTINGS.displayCurrency,
          barkDeviceKey: s.barkDeviceKey || DEFAULT_SETTINGS.barkDeviceKey,
          barkServerURL: s.barkServerURL || DEFAULT_SETTINGS.barkServerURL,
          barkEnabled: s.barkEnabled === "true",
          barkPriceAlert: s.barkPriceAlert !== "false",
          barkDriftAlert: s.barkDriftAlert !== "false",
          barkSummary: s.barkSummary !== "false",
          barkPriceThreshold:
            s.barkPriceThreshold != null
              ? Number(s.barkPriceThreshold)
              : DEFAULT_SETTINGS.barkPriceThreshold,
          barkSummaryInterval: s.barkSummaryInterval || DEFAULT_SETTINGS.barkSummaryInterval,
        })
      })
      .catch(console.error)
    api.fetchAvailableFunds(currentPortfolio.id).then(setAvailableFunds).catch(console.error)
    api.fetchFundTransactions(currentPortfolio.id).then(setFundTransactions).catch(console.error)
  }, [currentPortfolio])

  const handleSaveSettings = useCallback(
    async (newSettings: Settings) => {
      if (!currentPortfolio) throw new Error("未选择组合")
      try {
        await api.updateSettings(currentPortfolio.id, {
          driftThreshold: String(newSettings.driftThreshold),
          syncInterval: String(newSettings.syncInterval),
          colorScheme: newSettings.colorScheme,
          targetStocks: String(newSettings.targetStocks),
          targetBonds: String(newSettings.targetBonds),
          targetCash: String(newSettings.targetCash),
          targetCommodities: String(newSettings.targetCommodities),
          telegramBotToken: newSettings.telegramBotToken,
          telegramChatID: newSettings.telegramChatID,
          telegramEnabled: String(newSettings.telegramEnabled),
          telegramPriceAlert: String(newSettings.telegramPriceAlert),
          telegramDriftAlert: String(newSettings.telegramDriftAlert),
          telegramSummary: String(newSettings.telegramSummary),
          telegramPriceThreshold: String(newSettings.telegramPriceThreshold),
          telegramSummaryInterval: newSettings.telegramSummaryInterval,
          displayCurrency: newSettings.displayCurrency,
          barkDeviceKey: newSettings.barkDeviceKey,
          barkServerURL: newSettings.barkServerURL,
          barkEnabled: String(newSettings.barkEnabled),
          barkPriceAlert: String(newSettings.barkPriceAlert),
          barkDriftAlert: String(newSettings.barkDriftAlert),
          barkSummary: String(newSettings.barkSummary),
          barkPriceThreshold: String(newSettings.barkPriceThreshold),
          barkSummaryInterval: newSettings.barkSummaryInterval,
        })
        setSettings(newSettings)
      } catch (e) {
        console.error("Failed to save settings", e)
        throw e
      }
    },
    [currentPortfolio]
  )

  const handleRefreshAvailableFunds = useCallback(async () => {
    if (!currentPortfolio) return
    try {
      const [funds, txs] = await Promise.all([
        api.fetchAvailableFunds(currentPortfolio.id),
        api.fetchFundTransactions(currentPortfolio.id),
      ])
      setAvailableFunds(funds)
      setFundTransactions(txs)
    } catch (e) {
      console.error("Failed to refresh available funds", e)
    }
  }, [currentPortfolio])

  const handleAddHolding = useCallback(
    async (holding: Omit<Holding, "id">) => {
      await addHolding(holding)
      handleRefreshAvailableFunds()
    },
    [addHolding, handleRefreshAvailableFunds]
  )

  const handleRemoveHolding = useCallback(
    async (id: string) => {
      await removeHolding(id)
      handleRefreshAvailableFunds()
    },
    [removeHolding, handleRefreshAvailableFunds]
  )

  const handleTriggerSync = useCallback(async () => {
    if (!currentPortfolio) return
    try {
      const status = await api.triggerSync(currentPortfolio.id)
      setSyncStatus(status)
    } catch (e) {
      console.error("Failed to trigger sync", e)
    }
  }, [currentPortfolio])

  const handleSyncComplete = useCallback(
    (status: { lastSyncAt: string; lastSyncErr?: string; syncing: boolean }) => {
      setSyncStatus(status)
    },
    []
  )

  const handleLogout = useCallback(async () => {
    await api.logout()
    setUser(null)
  }, [])

  const handleShowSummary = useCallback(async () => {
    setShowSummary(true)
    setSummary(null)
    setSummaryError("")
    setSummaryLoading(true)
    try {
      const s = await api.fetchSummary()
      setSummary(s)
    } catch (e) {
      console.error("Failed to fetch summary", e)
      setSummaryError(e instanceof Error ? e.message : "汇总数据加载失败")
    } finally {
      setSummaryLoading(false)
    }
  }, [])

  if (setupMode === null) {
    return (
      <div className="min-h-screen bg-[#F8F9FA] flex items-center justify-center">
        <p className="text-sm text-[#6C757D]">Loading...</p>
      </div>
    )
  }

  if (setupMode) {
    return <SetupWizard onComplete={() => window.location.reload()} />
  }

  if (!authChecked) {
    return (
      <div className="min-h-screen bg-[#F8F9FA] flex items-center justify-center">
        <p className="text-sm text-[#6C757D]">Loading...</p>
      </div>
    )
  }

  if (!user) {
    return <LoginPage onLogin={() => window.location.reload()} />
  }

  if (!currentPortfolio) {
    return (
      <div className="min-h-screen bg-[#F8F9FA] flex items-center justify-center">
        <p className="text-sm text-[#6C757D]">加载投资组合中...</p>
      </div>
    )
  }

  if (loading) {
    return (
      <div className="min-h-screen bg-[#F8F9FA] flex items-center justify-center">
        <p className="text-sm text-[#6C757D]">Loading...</p>
      </div>
    )
  }

  const totalAssetsD = Object.values(assets).reduce(
    (sum, val) => sum.plus(toDecimal(val)),
    new Decimal(0)
  )
  const total = totalAssetsD.plus(totalFundsDisplay)
  const totalAssets = totalAssetsD.toString()
  const totalFees = holdings.reduce(
    (sum, h) =>
      sum.plus((h.lots || []).reduce((ls, l) => ls.plus(toDecimal(l.fee)), new Decimal(0))),
    new Decimal(0)
  )

  const byCurrency: Record<string, Decimal> = {}
  for (const tx of fundTransactions) {
    if (tx.type === "transfer_in") {
      byCurrency[tx.currency] = (byCurrency[tx.currency] || new Decimal(0)).plus(
        toDecimal(tx.amount)
      )
    } else if (tx.type === "transfer_out") {
      byCurrency[tx.currency] = (byCurrency[tx.currency] || new Decimal(0)).minus(
        toDecimal(tx.amount)
      )
    }
  }
  let principal = new Decimal(0)
  for (const [currency, amount] of Object.entries(byCurrency)) {
    if (currency === settings.displayCurrency || amount.isZero()) {
      principal = principal.plus(amount)
    } else {
      const rate = exchangeRates[currency]
      if (rate) principal = principal.plus(amount.times(rate))
    }
  }

  return (
    <div className="min-h-screen bg-[#F8F9FA] text-[#1A1A1A] font-sans flex flex-col overflow-x-hidden">
      <header className="h-20 bg-white border-b border-[#E9ECEF] flex items-center justify-between px-6 sm:px-10 shrink-0 lg:sticky lg:top-0 lg:z-10">
        <div className="flex items-center gap-3">
          <img src="/logo.svg" alt="投资组合管理" className="w-8 h-8 shrink-0" />
          <h1 className="text-xl font-semibold tracking-tight">投资组合管理</h1>
          <div
            role="tablist"
            aria-label="资产视图"
            className="relative grid grid-cols-2 items-center bg-[#F8F9FA] rounded-md p-0.5 ml-2"
          >
            <span
              aria-hidden="true"
              className={`absolute inset-y-0.5 left-0.5 w-[calc(50%-2px)] rounded bg-white shadow-sm transition-transform duration-300 ease-out motion-reduce:transition-none ${
                viewMode === "account" ? "translate-x-full" : "translate-x-0"
              }`}
            />
            <button
              type="button"
              role="tab"
              aria-selected={viewMode === "portfolio"}
              aria-controls="view-panel"
              onClick={() => setViewMode("portfolio")}
              className={`relative z-1 text-xs px-3 py-1 rounded transition-colors duration-200 ${
                viewMode === "portfolio" ? "text-[#1A1A1A]" : "text-[#6C757D] hover:text-[#1A1A1A]"
              }`}
            >
              组合视图
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={viewMode === "account"}
              aria-controls="view-panel"
              onClick={() => setViewMode("account")}
              className={`relative z-1 text-xs px-3 py-1 rounded transition-colors duration-200 ${
                viewMode === "account" ? "text-[#1A1A1A]" : "text-[#6C757D] hover:text-[#1A1A1A]"
              }`}
            >
              账户视图
            </button>
          </div>
          {viewMode === "portfolio" && (
            <PortfolioSelector
              portfolios={portfolios}
              current={currentPortfolio}
              onSelect={setCurrentPortfolio}
              onManage={() => setShowPortfolioManager(true)}
            />
          )}
          {viewMode === "account" && (
            <AccountSelector
              accounts={accounts}
              current={currentAccount}
              onSelect={setCurrentAccount}
              onManage={() => setShowAccountManager(true)}
            />
          )}
        </div>
        <div className="hidden sm:flex items-center gap-4">
          {syncStatus && syncStatus.lastSyncAt && (
            <button
              onClick={handleTriggerSync}
              disabled={syncStatus.syncing}
              className="text-[10px] text-[#6C757D] hover:text-[#1A1A1A] transition-colors disabled:opacity-50"
              title="手动同步价格"
            >
              {syncStatus.syncing
                ? "同步中..."
                : `上次同步: ${new Date(syncStatus.lastSyncAt).toLocaleTimeString()}`}
            </button>
          )}
          <button
            type="button"
            onClick={handleShowSummary}
            className="flex h-8 w-8 items-center justify-center rounded-lg text-[#6C757D] transition-colors hover:bg-[#F1F3F5] hover:text-[#1A1A1A]"
            title="查看投资组合汇总"
            aria-label="查看投资组合汇总"
          >
            <ChartPie className="h-4 w-4" />
          </button>
          <SettingsPanel
            settings={settings}
            onSave={handleSaveSettings}
            userRole={user.role}
            portfolioId={currentPortfolio.id}
          />
          {user.role === "admin" && <UserManager currentUser={user} />}
          <div className="flex items-center gap-2">
            <span className="text-xs text-[#6C757D]">{user.username}</span>
            <button
              onClick={handleLogout}
              className="text-xs text-[#6C757D] hover:text-[#1A1A1A] transition-colors"
            >
              退出
            </button>
          </div>
        </div>
      </header>

      <main
        id="view-panel"
        role="tabpanel"
        className="grow p-4 sm:p-8 flex flex-col gap-8 max-w-350 mx-auto w-full"
      >
        {viewMode === "account" ? (
          <div key="account" className="view-content-enter view-content-enter-from-right">
            <AccountView
              selectedAccount={currentAccount}
              colorScheme={settings.colorScheme}
              portfolios={portfolios}
              currentPortfolio={currentPortfolio}
              accounts={accounts}
              onAddHolding={handleAddHolding}
              onRefreshAvailableFunds={handleRefreshAvailableFunds}
            />
          </div>
        ) : (
          <div
            key="portfolio"
            className="view-content-enter view-content-enter-from-left flex flex-col gap-8"
          >
            <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
              <div className="lg:col-span-5 flex flex-col gap-6 h-full">
                <Dashboard
                  assets={assets}
                  total={total.toString()}
                  totalAssets={totalAssets}
                  principal={principal.toString()}
                  totalFees={totalFees.toString()}
                  colorScheme={settings.colorScheme}
                  availableFunds={availableFunds}
                  exchangeRates={exchangeRates}
                  portfolios={portfolios}
                  currentPortfolioId={currentPortfolio.id}
                  onRefreshFunds={handleRefreshAvailableFunds}
                  displayCurrency={settings.displayCurrency}
                />
              </div>
              <div className="lg:col-span-7 flex flex-col gap-6 h-full">
                <RebalancePanel
                  assets={assets}
                  total={total.toString()}
                  driftThreshold={settings.driftThreshold}
                  colorScheme={settings.colorScheme}
                  targetPcts={{
                    stocks: settings.targetStocks,
                    bonds: settings.targetBonds,
                    cash: settings.targetCash,
                    commodities: settings.targetCommodities,
                  }}
                  displayCurrency={settings.displayCurrency}
                />
              </div>
            </div>

            <div className="flex flex-col gap-6">
              <HoldingsManager
                portfolioId={currentPortfolio.id}
                holdings={holdings}
                setHoldings={setHoldings}
                total={total.toString()}
                onAddHolding={handleAddHolding}
                onUpdateHolding={updateHolding}
                onRemoveHolding={handleRemoveHolding}
                onSaveRecord={saveRecord}
                colorScheme={settings.colorScheme}
                displayCurrency={settings.displayCurrency}
                onRefreshAvailableFunds={handleRefreshAvailableFunds}
                onSyncComplete={handleSyncComplete}
                accounts={accounts}
              />
              <HistoryPanel
                history={history}
                onDeleteRecord={deleteRecord}
                colorScheme={settings.colorScheme}
                displayCurrency={settings.displayCurrency}
              />
            </div>
          </div>
        )}
      </main>

      {showPortfolioManager && (
        <PortfolioManager
          portfolios={portfolios}
          onClose={() => setShowPortfolioManager(false)}
          onRefresh={loadPortfolios}
        />
      )}
      {showAccountManager && (
        <AccountManager
          accounts={accounts}
          onClose={() => setShowAccountManager(false)}
          onRefresh={loadAccounts}
        />
      )}
      {showSummary && (
        <SummaryDashboard
          summary={summary}
          colorScheme={settings.colorScheme}
          displayCurrency={settings.displayCurrency}
          loading={summaryLoading}
          error={summaryError}
          onRetry={handleShowSummary}
          onClose={() => setShowSummary(false)}
        />
      )}
    </div>
  )
}
