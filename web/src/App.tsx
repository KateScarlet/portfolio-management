import { useState, useEffect, useCallback, useRef } from "react"
import { usePortfolio } from "./usePortfolio"
import { useExchangeRates } from "./useExchangeRates"
import {
  Settings,
  SyncStatus,
  UserInfo,
  Portfolio,
  PortfolioSummary,
  DEFAULT_SETTINGS,
  ColorScheme,
  Holding,
  AvailableFund,
} from "./types"
import * as api from "./api"
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

export default function App() {
  const [setupMode, setSetupMode] = useState<boolean | null>(null)
  const [user, setUser] = useState<UserInfo | null>(null)
  const [authChecked, setAuthChecked] = useState(false)
  const [portfolios, setPortfolios] = useState<Portfolio[]>([])
  const [currentPortfolio, setCurrentPortfolio] = useState<Portfolio | null>(null)
  const [showPortfolioManager, setShowPortfolioManager] = useState(false)
  const [showSummary, setShowSummary] = useState(false)
  const [summary, setSummary] = useState<PortfolioSummary | null>(null)
  const [settings, setSettings] = useState<Settings>(DEFAULT_SETTINGS)
  const [syncStatus, setSyncStatus] = useState<SyncStatus | null>(null)
  const [availableFunds, setAvailableFunds] = useState<AvailableFund[]>([])
  const exchangeRates = useExchangeRates(availableFunds)

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
  } = usePortfolio(currentPortfolio?.id || null, settings.displayCurrency)

  const totalFundsCNY = availableFunds.reduce((sum, f) => {
    const rate = exchangeRates[f.currency]
    return rate ? sum + f.amount * rate : sum
  }, 0)

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
        const existing = prev ? ps.find((p) => p.id === prev.id) : null
        return existing || ps[0]
      })
    } catch (e) {
      console.error("Failed to load portfolios", e)
    }
  }, [])

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
            const existing = prev ? ps.find((p) => p.id === prev.id) : null
            return existing || ps[0]
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
  }, [user])

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
              : (s as any).targetGold != null
                ? Number((s as any).targetGold)
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
    api.fetchSyncStatus(currentPortfolio.id).then(setSyncStatus).catch(console.error)
  }, [currentPortfolio])

  const prevSyncingRef = useRef(false)
  const syncPollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const pollSyncStatusRef = useRef<() => void>(() => {})

  const pollSyncStatus = useCallback(() => {
    if (!currentPortfolio) return
    api
      .fetchSyncStatus(currentPortfolio.id)
      .then((status) => {
        if (prevSyncingRef.current && !status.syncing && currentPortfolio) {
          api.fetchHoldings(currentPortfolio.id).then(setHoldings).catch(console.error)
        }
        prevSyncingRef.current = status.syncing
        setSyncStatus(status)
        if (syncPollRef.current) clearInterval(syncPollRef.current)
        syncPollRef.current = setInterval(pollSyncStatusRef.current, status.syncing ? 2000 : 30000)
      })
      .catch(console.error)
  }, [setHoldings, currentPortfolio])

  useEffect(() => {
    pollSyncStatusRef.current = pollSyncStatus
  }, [pollSyncStatus])

  useEffect(() => {
    if (!user || !currentPortfolio) return
    syncPollRef.current = setInterval(pollSyncStatus, 30000)
    return () => {
      if (syncPollRef.current) clearInterval(syncPollRef.current)
    }
  }, [user, currentPortfolio, pollSyncStatus])

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
      const funds = await api.fetchAvailableFunds(currentPortfolio.id)
      setAvailableFunds(funds)
    } catch (e) {
      console.error("Failed to refresh available funds", e)
    }
  }, [currentPortfolio])

  const handleAddHolding = useCallback(
    async (holding: Omit<Holding, "id">) => {
      await addHolding(holding)
      if (holding.deductFromCash) {
        handleRefreshAvailableFunds()
      }
    },
    [addHolding, handleRefreshAvailableFunds]
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

  const total = Object.values(assets).reduce((sum, val) => sum + val, 0) + totalFundsCNY
  const totalAssets = Object.values(assets).reduce((sum, val) => sum + val, 0)
  const totalCost = holdings.reduce((sum, h) => sum + (h.cost || 0), 0)
  const totalFees = holdings.reduce(
    (sum, h) => sum + (h.lots || []).reduce((ls, l) => ls + (l.fee || 0), 0),
    0
  )
  const totalBuyFees = holdings.reduce(
    (sum, h) =>
      sum + (h.lots || []).reduce((ls, l) => ls + (l.type !== "sell" ? l.fee || 0 : 0), 0),
    0
  )
  const principal = totalCost + totalBuyFees

  return (
    <div className="min-h-screen bg-[#F8F9FA] text-[#1A1A1A] font-sans flex flex-col overflow-x-hidden">
      <header className="h-20 bg-white border-b border-[#E9ECEF] flex items-center justify-between px-6 sm:px-10 shrink-0 lg:sticky lg:top-0 lg:z-10">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 bg-[#1A1A1A] rounded-md flex items-center justify-center">
            <div className="w-4 h-4 border-2 border-white rounded-full"></div>
          </div>
          <h1 className="text-xl font-semibold tracking-tight">投资组合管理</h1>
          <PortfolioSelector
            portfolios={portfolios}
            current={currentPortfolio}
            onSelect={setCurrentPortfolio}
            onManage={() => setShowPortfolioManager(true)}
          />
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
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
          <div className="lg:col-span-5 flex flex-col gap-6 h-full">
            <Dashboard
              assets={assets}
              total={total}
              totalAssets={totalAssets}
              principal={principal}
              totalFees={totalFees}
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
              total={total}
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
            total={total}
            onAddHolding={handleAddHolding}
            onUpdateHolding={updateHolding}
            onRemoveHolding={removeHolding}
            onSaveRecord={saveRecord}
            colorScheme={settings.colorScheme}
            onRefreshAvailableFunds={handleRefreshAvailableFunds}
            onSyncComplete={handleSyncComplete}
          />
          <HistoryPanel
            history={history}
            onDeleteRecord={deleteRecord}
            colorScheme={settings.colorScheme}
            displayCurrency={settings.displayCurrency}
          />
        </div>
      </main>

      {showPortfolioManager && (
        <PortfolioManager
          portfolios={portfolios}
          onClose={() => setShowPortfolioManager(false)}
          onRefresh={loadPortfolios}
        />
      )}
      {showSummary && (
        <SummaryDashboard
          summary={summary}
          colorScheme={settings.colorScheme}
          onClose={() => setShowSummary(false)}
        />
      )}
    </div>
  )
}
