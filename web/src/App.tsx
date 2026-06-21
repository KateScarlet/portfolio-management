import { useState, useEffect, useCallback } from "react"
import { usePortfolio } from "./usePortfolio"
import { Settings, SyncStatus, DEFAULT_SETTINGS } from "./types"
import * as api from "./api"
import Dashboard from "./components/Dashboard"
import HoldingsManager from "./components/HoldingsManager"
import RebalancePanel from "./components/RebalancePanel"
import HistoryPanel from "./components/HistoryPanel"
import SettingsPanel from "./components/SettingsPanel"

export default function App() {
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
  } = usePortfolio()
  const [settings, setSettings] = useState<Settings>(DEFAULT_SETTINGS)
  const [syncStatus, setSyncStatus] = useState<SyncStatus | null>(null)

  useEffect(() => {
    api
      .fetchSettings()
      .then((s) => {
        setSettings({
          driftThreshold: s.driftThreshold != null ? Number(s.driftThreshold) : DEFAULT_SETTINGS.driftThreshold,
          syncInterval: s.syncInterval != null ? Number(s.syncInterval) : DEFAULT_SETTINGS.syncInterval,
        })
      })
      .catch(console.error)
    api.fetchSyncStatus().then(setSyncStatus).catch(console.error)
  }, [])

  // Poll sync status every 30s
  useEffect(() => {
    const interval = setInterval(() => {
      api.fetchSyncStatus().then(setSyncStatus).catch(console.error)
    }, 30000)
    return () => clearInterval(interval)
  }, [])

  const handleSaveSettings = useCallback(async (newSettings: Settings) => {
    try {
      await api.updateSettings({
        driftThreshold: String(newSettings.driftThreshold),
        syncInterval: String(newSettings.syncInterval),
      })
      setSettings(newSettings)
    } catch (e) {
      console.error("Failed to save settings", e)
    }
  }, [])

  const handleTriggerSync = useCallback(async () => {
    try {
      const status = await api.triggerSync()
      setSyncStatus(status)
    } catch (e) {
      console.error("Failed to trigger sync", e)
    }
  }, [])

  const total = Object.values(assets).reduce((sum, val) => sum + val, 0)
  const totalCost = holdings.reduce((sum, h) => sum + (h.cost || 0), 0)
  const totalFees = holdings.reduce(
    (sum, h) => sum + (h.lots || []).reduce((ls, l) => ls + (l.fee || 0), 0),
    0
  )
  // Buy-only fees for principal: sell fees are already deducted from
  // realizedValue, so including them in principal would double-count.
  const totalBuyFees = holdings.reduce(
    (sum, h) =>
      sum +
      (h.lots || []).reduce((ls, l) => ls + (l.type !== "sell" ? (l.fee || 0) : 0), 0),
    0
  )
  // principal = cost of current holdings + buy fees only
  const principal = totalCost + totalBuyFees

  if (loading) {
    return (
      <div className="min-h-screen bg-[#F8F9FA] flex items-center justify-center">
        <p className="text-sm text-[#6C757D]">Loading...</p>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-[#F8F9FA] text-[#1A1A1A] font-sans flex flex-col overflow-x-hidden">
      {/* Header */}
      <header className="h-20 bg-white border-b border-[#E9ECEF] flex items-center justify-between px-6 sm:px-10 flex-shrink-0 lg:sticky lg:top-0 lg:z-10">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 bg-[#1A1A1A] rounded-md flex items-center justify-center">
            <div className="w-4 h-4 border-2 border-white rounded-full"></div>
          </div>
          <h1 className="text-xl font-semibold tracking-tight">投资组合</h1>
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
          <SettingsPanel settings={settings} onSave={handleSaveSettings} />
        </div>
      </header>

      {/* Main Content Grid */}
      <main className="flex-grow p-4 sm:p-8 flex flex-col gap-8 max-w-[1400px] mx-auto w-full">
        {/* Top Row: Dashboard & Rebalance */}
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
          <div className="lg:col-span-5 flex flex-col gap-6 h-full">
            <Dashboard assets={assets} total={total} principal={principal} totalFees={totalFees} />
          </div>
          <div className="lg:col-span-7 flex flex-col gap-6 h-full">
            <RebalancePanel
              assets={assets}
              total={total}
              driftThreshold={settings.driftThreshold}
            />
          </div>
        </div>

        {/* Bottom Row: Holdings & History */}
        <div className="flex flex-col gap-6">
          <HoldingsManager
            holdings={holdings}
            setHoldings={setHoldings}
            total={total}
            onAddHolding={addHolding}
            onUpdateHolding={updateHolding}
            onRemoveHolding={removeHolding}
            onSaveRecord={saveRecord}
          />
          <HistoryPanel history={history} onDeleteRecord={deleteRecord} />
        </div>
      </main>
    </div>
  )
}
