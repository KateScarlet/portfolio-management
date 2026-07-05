import { useState, useEffect, useCallback } from "react"
import Decimal from "decimal.js"
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

export default function App() {
  const [setupMode, setSetupMode] = useState<boolean | null>(null)
  const [user, setUser] = useState<UserInfo | null>(null)
  const [authChecked, setAuthChecked] = useState(false)
  const [portfolios, setPortfolios] = useState<Portfolio[]>([])
  const [currentPortfolio, setCurrentPortfolioState] = useState<Portfolio | null>(null)
  const [showPortfolioManager, setShowPortfolioManager] = useState(false)
  const [viewMode, setViewMode] = useState<"portfolio" | "account">("portfolio")
  const [accounts, setAccounts] = useState<Account[]>([])
  const [currentAccount, setCurrentAccount] = useState<Account | null>(null)
  const [showAccountManager, setShowAccountManager] = useState(false)

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
      if (!prev) return null
      return list.find((a) => a.id === prev.id) || null
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
        })
      })
      .catch(console.error)
    api.fetchAvailableFunds(currentPortfolio.id).then(setAvailableFunds).catch(console.error)
    api.fetchFundTransactions(currentPortfolio.id).then(setFundTransactions).catch(console.error)
  }, [currentPortfolio])

  const handleSaveSettings = useCallback(
    async (newSettings: Settings) => {
      if (!currentPortfolio) return
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
        })
        setSettings(newSettings)
      } catch (e) {
        console.error("Failed to save settings", e)
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
    try {
      const s = await api.fetchSummary()
      setSummary(s)
      setShowSummary(true)
    } catch (e) {
      console.error("Failed to fetch summary", e)
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
          <div className="w-8 h-8 bg-[#1A1A1A] rounded-md flex items-center justify-center">
            <div className="w-4 h-4 border-2 border-white rounded-full"></div>
          </div>
          <h1 className="text-xl font-semibold tracking-tight">投资组合管理</h1>
          <div className="flex items-center bg-[#F8F9FA] rounded-md p-0.5 ml-2">
            <button
              onClick={() => setViewMode("portfolio")}
              className={`text-xs px-3 py-1 rounded transition-colors ${
                viewMode === "portfolio"
                  ? "bg-white text-[#1A1A1A] shadow-sm"
                  : "text-[#6C757D] hover:text-[#1A1A1A]"
              }`}
            >
              组合视图
            </button>
            <button
              onClick={() => setViewMode("account")}
              className={`text-xs px-3 py-1 rounded transition-colors ${
                viewMode === "account"
                  ? "bg-white text-[#1A1A1A] shadow-sm"
                  : "text-[#6C757D] hover:text-[#1A1A1A]"
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
            onClick={handleShowSummary}
            className="text-xs text-[#6C757D] hover:text-[#1A1A1A] transition-colors"
            title="查看汇总"
          >
            汇总
          </button>
          <SettingsPanel settings={settings} onSave={handleSaveSettings} userRole={user.role} />
          {user.role === "admin" && <UserManager />}
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

      <main className="grow p-4 sm:p-8 flex flex-col gap-8 max-w-350 mx-auto w-full">
        {viewMode === "account" ? (
          <AccountView
            selectedAccount={currentAccount}
            colorScheme={settings.colorScheme}
            displayCurrency={settings.displayCurrency}
            portfolios={portfolios}
            currentPortfolio={currentPortfolio}
            accounts={accounts}
            onAddHolding={handleAddHolding}
            onRefreshAvailableFunds={handleRefreshAvailableFunds}
          />
        ) : (
          <>
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
          </>
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
          onClose={() => setShowSummary(false)}
        />
      )}
    </div>
  )
}
