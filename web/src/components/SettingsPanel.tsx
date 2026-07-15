import { useState, useEffect, useRef, useCallback } from "react"
import {
  Settings,
  AssetId,
  ASSET_DEFINITIONS,
  MARKET_OPTIONS,
  MarketSourceConfig,
  SourceTestResult,
  SourceTestComplete,
} from "../types"
import {
  Settings as SettingsIcon,
  ArrowUp,
  ArrowDown,
  Target,
  RefreshCw,
  Palette,
  Bell,
  Shield,
  Check,
  X,
  Loader2,
} from "lucide-react"
import * as api from "../api"
import { useToast } from "./toast-context"

interface SettingsPanelProps {
  settings: Settings
  onSave: (settings: Settings) => void | Promise<void>
  userRole: "admin" | "user"
  portfolioId: string
}

const SYNC_PRESETS = [
  { value: 0, label: "关闭" },
  { value: 10, label: "10分钟" },
  { value: 30, label: "30分钟" },
  { value: 60, label: "1小时" },
  { value: 120, label: "2小时" },
  { value: 240, label: "4小时" },
]

const SUMMARY_INTERVALS = [
  { value: "off", label: "关闭" },
  { value: "daily", label: "每日" },
  { value: "weekly", label: "每周" },
]

const DISPLAY_CURRENCIES = [
  { value: "CNY", label: "CNY ¥" },
  { value: "USD", label: "USD $" },
  { value: "HKD", label: "HKD $" },
  { value: "EUR", label: "EUR €" },
  { value: "JPY", label: "JPY ¥" },
  { value: "GBP", label: "GBP £" },
]

const PORTFOLIO_SECTIONS = [
  { id: "invest", label: "投资", icon: Target },
  { id: "sync", label: "同步", icon: RefreshCw },
  { id: "display", label: "显示", icon: Palette },
  { id: "notify", label: "通知", icon: Bell },
]

const USER_SECTIONS = [
  { id: "sources", label: "行情源", icon: RefreshCw },
  { id: "security", label: "安全", icon: Shield },
]

const SYSTEM_SECTIONS = [{ id: "system-security", label: "认证", icon: Shield }]

type SettingsScope = "portfolio" | "user" | "system"

export default function SettingsPanel({
  settings,
  onSave,
  userRole,
  portfolioId,
}: SettingsPanelProps) {
  const { showToast } = useToast()
  const [isOpen, setIsOpen] = useState(false)
  const [activeScope, setActiveScope] = useState<SettingsScope>("portfolio")
  const [activeSection, setActiveSection] = useState("invest")
  const [draft, setDraft] = useState(settings)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null)
  const [testType, setTestType] = useState<"connection" | "price" | "drift" | "summary">(
    "connection"
  )
  const [barkTesting, setBarkTesting] = useState(false)
  const [barkTestResult, setBarkTestResult] = useState<{
    success: boolean
    message: string
  } | null>(null)
  const [barkTestType, setBarkTestType] = useState<"connection" | "price" | "drift" | "summary">(
    "connection"
  )
  const [oidcConfig, setOidcConfig] = useState<api.OIDCConfig | null>(null)
  const [oidcDraft, setOidcDraft] = useState<api.OIDCConfig>({
    enabled: false,
    issuer: "",
    clientID: "",
    clientSecret: "",
    redirectURL: "",
  })
  const [webauthnDraft, setWebauthnDraft] = useState<{
    enabled: boolean
    rpid: string
    rpOrigins: string
  }>({ enabled: false, rpid: "", rpOrigins: "" })
  const [marketSources, setMarketSources] = useState<MarketSourceConfig | null>(null)
  const [marketSourceDraft, setMarketSourceDraft] = useState<Record<string, string[]>>({})
  const [dragState, setDragState] = useState<{ market: string; src: string } | null>(null)
  const [dropTarget, setDropTarget] = useState<{ market: string; src: string } | null>(null)
  const [testingSources, setTestingSources] = useState(false)
  const [sourceTestResults, setSourceTestResults] = useState<Record<string, SourceTestResult>>({})
  const [sourceTestResultsOrder, setSourceTestResultsOrder] = useState<string[]>([])
  const [testProgress, setTestProgress] = useState<{
    tested: number
    total: number
    success: number
  } | null>(null)

  const handleOpen = () => {
    setDraft(settings)
    setActiveScope("portfolio")
    setActiveSection("invest")
    setIsOpen(true)
    if (userRole === "admin") {
      api
        .fetchOIDCConfig()
        .then((config) => {
          setOidcConfig(config)
          setOidcDraft(config)
        })
        .catch(() => {})
      api
        .fetchWebAuthnConfig()
        .then((config) => {
          setWebauthnDraft({
            enabled: config.enabled,
            rpid: config.rpid,
            rpOrigins: config.rpOrigins.join(", "),
          })
        })
        .catch(() => {})
    }
    api
      .fetchMarketSources()
      .then((ms) => {
        setMarketSources(ms)
        setMarketSourceDraft(ms.config ?? {})
      })
      .catch(() => {})
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      await onSave(draft)
      if (userRole === "admin") {
        if (oidcConfig) {
          const result = await api.updateOIDCConfig(oidcDraft)
          setOidcConfig(result)
          setOidcDraft(result)
        }
        const origins = webauthnDraft.rpOrigins
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean)
        await api.updateWebAuthnConfig({
          enabled: webauthnDraft.enabled,
          rpid: webauthnDraft.rpid,
          rpOrigins: origins,
        })
      }
      await api.updateMarketSources(marketSourceDraft)
      setIsOpen(false)
      setTestResult(null)
      showToast("设置保存成功", "success")
    } catch (e) {
      console.error("Failed to save settings", e)
      showToast(e instanceof Error ? `设置保存失败：${e.message}` : "设置保存失败", "error")
    } finally {
      setSaving(false)
    }
  }

  const handleTestConnection = async () => {
    if (!draft.telegramBotToken || !draft.telegramChatID) {
      setTestResult({ success: false, message: "请先填写 Bot Token 和 Chat ID" })
      return
    }
    setTesting(true)
    setTestResult(null)
    try {
      const result = await api.testTelegramConnection(draft.telegramBotToken, draft.telegramChatID)
      if (result.success) {
        setTestResult({ success: true, message: `连接成功！Bot: @${result.botName}` })
      } else {
        setTestResult({ success: false, message: result.error || "连接失败" })
      }
    } catch (e) {
      setTestResult({
        success: false,
        message: "连接失败: " + (e instanceof Error ? e.message : "未知错误"),
      })
    } finally {
      setTesting(false)
    }
  }

  const handleTestMessage = async (type: "price" | "drift" | "summary") => {
    if (!draft.telegramBotToken || !draft.telegramChatID) {
      setTestResult({ success: false, message: "请先填写 Bot Token 和 Chat ID" })
      return
    }
    setTesting(true)
    setTestResult(null)
    try {
      const result = await api.testTelegramMessage(
        draft.telegramBotToken,
        draft.telegramChatID,
        type,
        portfolioId
      )
      if (result.success) {
        const labels = { price: "价格告警", drift: "配比偏离", summary: "组合摘要" }
        setTestResult({ success: true, message: `已发送${labels[type]}测试消息` })
      } else {
        setTestResult({ success: false, message: result.error || "发送失败" })
      }
    } catch (e) {
      setTestResult({
        success: false,
        message: "发送失败: " + (e instanceof Error ? e.message : "未知错误"),
      })
    } finally {
      setTesting(false)
    }
  }

  const handleBarkTestConnection = async () => {
    if (!draft.barkDeviceKey) {
      setBarkTestResult({ success: false, message: "请先填写 Device Key" })
      return
    }
    setBarkTesting(true)
    setBarkTestResult(null)
    try {
      const result = await api.testBarkConnection(draft.barkDeviceKey, draft.barkServerURL)
      if (result.success) {
        setBarkTestResult({ success: true, message: "连接成功！" })
      } else {
        setBarkTestResult({ success: false, message: result.error || "连接失败" })
      }
    } catch (e) {
      setBarkTestResult({
        success: false,
        message: "连接失败: " + (e instanceof Error ? e.message : "未知错误"),
      })
    } finally {
      setBarkTesting(false)
    }
  }

  const handleBarkTestMessage = async (type: "price" | "drift" | "summary") => {
    if (!draft.barkDeviceKey) {
      setBarkTestResult({ success: false, message: "请先填写 Device Key" })
      return
    }
    setBarkTesting(true)
    setBarkTestResult(null)
    try {
      const result = await api.testBarkMessage(
        draft.barkDeviceKey,
        draft.barkServerURL,
        type,
        portfolioId
      )
      if (result.success) {
        const labels = { price: "价格告警", drift: "配比偏离", summary: "组合摘要" }
        setBarkTestResult({ success: true, message: `已发送${labels[type]}测试消息` })
      } else {
        setBarkTestResult({ success: false, message: result.error || "发送失败" })
      }
    } catch (e) {
      setBarkTestResult({
        success: false,
        message: "发送失败: " + (e instanceof Error ? e.message : "未知错误"),
      })
    } finally {
      setBarkTesting(false)
    }
  }

  const presets = [3, 5, 7, 10, 15, 20]

  const visibleSections =
    activeScope === "portfolio"
      ? PORTFOLIO_SECTIONS
      : activeScope === "user"
        ? USER_SECTIONS
        : SYSTEM_SECTIONS

  const handleScopeChange = (scope: SettingsScope) => {
    setActiveScope(scope)
    setActiveSection(
      scope === "portfolio" ? "invest" : scope === "user" ? "sources" : "system-security"
    )
  }

  return (
    <>
      <button
        type="button"
        onClick={handleOpen}
        className="flex h-8 w-8 items-center justify-center rounded-lg text-[#6C757D] transition-colors hover:bg-[#F1F3F5] hover:text-[#1A1A1A]"
        title="设置"
        aria-label="设置"
      >
        <SettingsIcon className="h-4 w-4" />
      </button>

      {isOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/20"
          onClick={() => setIsOpen(false)}
        >
          <div
            className="bg-white rounded-2xl shadow-xl w-full max-w-2xl mx-4 max-h-[80vh] flex flex-col"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Fixed Header */}
            <div className="flex items-center justify-between px-6 pt-6 pb-4">
              <h3 className="text-lg font-medium text-[#1A1A1A]">设置</h3>
              <button
                onClick={() => setIsOpen(false)}
                className="text-[#ADB5BD] hover:text-[#1A1A1A] text-xl leading-none"
              >
                &times;
              </button>
            </div>

            <div className="flex gap-1 mx-6 mb-4 p-1 rounded-lg bg-[#F1F3F5]" role="tablist">
              <button
                role="tab"
                aria-selected={activeScope === "portfolio"}
                onClick={() => handleScopeChange("portfolio")}
                className={`flex-1 px-3 py-1.5 text-sm rounded-md transition-colors ${
                  activeScope === "portfolio"
                    ? "bg-white text-[#1A1A1A] shadow-sm"
                    : "text-[#6C757D] hover:text-[#1A1A1A]"
                }`}
              >
                组合设置
              </button>
              <button
                role="tab"
                aria-selected={activeScope === "user"}
                onClick={() => handleScopeChange("user")}
                className={`flex-1 px-3 py-1.5 text-sm rounded-md transition-colors ${
                  activeScope === "user"
                    ? "bg-white text-[#1A1A1A] shadow-sm"
                    : "text-[#6C757D] hover:text-[#1A1A1A]"
                }`}
              >
                用户设置
              </button>
              {userRole === "admin" && (
                <button
                  role="tab"
                  aria-selected={activeScope === "system"}
                  onClick={() => handleScopeChange("system")}
                  className={`flex-1 px-3 py-1.5 text-sm rounded-md transition-colors ${
                    activeScope === "system"
                      ? "bg-white text-[#1A1A1A] shadow-sm"
                      : "text-[#6C757D] hover:text-[#1A1A1A]"
                  }`}
                >
                  系统设置
                </button>
              )}
            </div>

            {/* Body: sidebar + content */}
            <div className="flex flex-1 min-h-0">
              {/* Sidebar */}
              <nav className="w-36 shrink-0 border-r border-[#E9ECEF] px-2 pt-1 pb-4 overflow-y-auto">
                {visibleSections.map((s) => {
                  const Icon = s.icon
                  return (
                    <button
                      key={s.id}
                      onClick={() => setActiveSection(s.id)}
                      className={`w-full flex items-center gap-2 px-3 py-2 text-sm rounded-lg transition-colors mb-0.5 ${
                        activeSection === s.id
                          ? "bg-[#1A1A1A] text-white"
                          : "text-[#6C757D] hover:text-[#1A1A1A] hover:bg-[#F1F3F5]"
                      }`}
                    >
                      <Icon className="w-4 h-4 shrink-0" />
                      {s.label}
                    </button>
                  )
                })}
              </nav>

              {/* Content */}
              <div className="flex-1 overflow-y-auto scrollbar-thin px-6 pb-2 pt-1">
                {/* Invest */}
                {activeSection === "invest" && (
                  <div className="space-y-6">
                    <div>
                      <label className="block text-sm font-medium text-[#1A1A1A] mb-2">
                        再平衡漂移阈值
                      </label>
                      <p className="text-xs text-[#6C757D] mb-3">
                        当资产偏离目标配比超过此阈值时，提示需要再平衡。
                      </p>
                      <div className="flex items-center gap-3">
                        <input
                          type="range"
                          min="1"
                          max="30"
                          step="1"
                          value={draft.driftThreshold}
                          onChange={(e) =>
                            setDraft({ ...draft, driftThreshold: Number(e.target.value) })
                          }
                          className="flex-1 h-2 bg-[#E9ECEF] rounded-lg appearance-none cursor-pointer accent-[#1A1A1A]"
                        />
                        <div className="flex items-center gap-1 w-20">
                          <input
                            type="number"
                            min="1"
                            max="30"
                            value={draft.driftThreshold}
                            onChange={(e) =>
                              setDraft({
                                ...draft,
                                driftThreshold: Math.max(
                                  1,
                                  Math.min(30, Number(e.target.value) || 1)
                                ),
                              })
                            }
                            className="w-14 px-2 py-1.5 text-sm text-center border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                          />
                          <span className="text-xs text-[#6C757D]">%</span>
                        </div>
                      </div>
                      <div className="flex flex-wrap gap-2 mt-3">
                        {presets.map((p) => (
                          <button
                            key={p}
                            onClick={() => setDraft({ ...draft, driftThreshold: p })}
                            className={`px-3 py-1 text-xs rounded-full border transition-colors ${
                              draft.driftThreshold === p
                                ? "bg-[#1A1A1A] text-white border-[#1A1A1A]"
                                : "bg-white text-[#6C757D] border-[#E9ECEF] hover:border-[#ADB5BD]"
                            }`}
                          >
                            {p}%
                          </button>
                        ))}
                      </div>
                    </div>

                    <div>
                      <label className="block text-sm font-medium text-[#1A1A1A] mb-2">
                        目标配比
                      </label>
                      <p className="text-xs text-[#6C757D] mb-3">
                        拖动分隔线调整各资产类别目标占比，用于再平衡建议和偏离提醒。
                      </p>
                      <TargetAllocationBar
                        draft={draft}
                        onChange={(patch) => setDraft({ ...draft, ...patch })}
                      />
                    </div>
                  </div>
                )}

                {/* Sync */}
                {activeSection === "sync" && (
                  <div className="space-y-6">
                    <div>
                      <label className="block text-sm font-medium text-[#1A1A1A] mb-2">
                        自动同步价格
                      </label>
                      <p className="text-xs text-[#6C757D] mb-3">定时从数据源获取最新价格。</p>
                      <div className="flex flex-wrap gap-2">
                        {SYNC_PRESETS.map((p) => (
                          <button
                            key={p.value}
                            onClick={() => setDraft({ ...draft, syncInterval: p.value })}
                            className={`px-3 py-1 text-xs rounded-full border transition-colors ${
                              draft.syncInterval === p.value
                                ? "bg-[#1A1A1A] text-white border-[#1A1A1A]"
                                : "bg-white text-[#6C757D] border-[#E9ECEF] hover:border-[#ADB5BD]"
                            }`}
                          >
                            {p.label}
                          </button>
                        ))}
                      </div>
                    </div>
                  </div>
                )}

                {/* Market sources */}
                {activeSection === "sources" && (
                  <div className="space-y-6">
                    {marketSources && (
                      <div>
                        <div className="flex items-center justify-between mb-2">
                          <label className="text-sm font-medium text-[#1A1A1A]">行情源配置</label>
                          <button
                            onClick={() => {
                              setTestingSources(true)
                              setSourceTestResults({})
                              setSourceTestResultsOrder([])
                              setTestProgress(null)
                              api.testMarketSourcesStream(marketSourceDraft, {
                                onResult: (key, _source, _market, result) => {
                                  setSourceTestResults((prev) => ({ ...prev, [key]: result }))
                                  setSourceTestResultsOrder((prev) => [...prev, key])
                                  setTestProgress((prev) =>
                                    prev ? { ...prev, tested: prev.tested + 1 } : null
                                  )
                                },
                                onComplete: (summary: SourceTestComplete) => {
                                  setTestProgress({
                                    tested: summary.total,
                                    total: summary.total,
                                    success: summary.success,
                                  })
                                  setTestingSources(false)
                                },
                                onError: (err) => {
                                  console.error("Test failed", err)
                                  setTestingSources(false)
                                },
                              })
                            }}
                            disabled={testingSources}
                            className="flex items-center gap-1 px-2 py-1 text-xs rounded border transition-colors text-[#6C757D] hover:text-[#1A1A1A] hover:border-[#ADB5BD] disabled:opacity-50"
                          >
                            {testingSources ? (
                              <Loader2 className="w-3 h-3 animate-spin" />
                            ) : (
                              <RefreshCw className="w-3 h-3" />
                            )}
                            测试
                          </button>
                        </div>
                        <p className="text-xs text-[#6C757D] mb-3">
                          拖动已选源调整优先级，点击取消选中，点击未选源添加。
                        </p>
                        <div className="space-y-3">
                          {MARKET_OPTIONS.map((m) => {
                            const available = marketSources.available[m.code] || []
                            const selected = marketSourceDraft[m.code] || []
                            return (
                              <div key={m.code}>
                                <div className="flex items-center gap-3">
                                  <span className="text-xs text-[#495057] w-20 shrink-0">
                                    {m.name}
                                  </span>
                                  <div className="flex flex-wrap gap-1.5">
                                    {[
                                      ...selected,
                                      ...available.filter((s) => !selected.includes(s)),
                                    ].map((src) => {
                                      const isSelected = selected.includes(src)
                                      const isDragging =
                                        dragState?.market === m.code && dragState?.src === src
                                      const isDrop =
                                        dropTarget?.market === m.code && dropTarget?.src === src
                                      return (
                                        <div key={src} className="flex items-center gap-1">
                                          <button
                                            draggable={isSelected && selected.length > 1}
                                            onClick={() => {
                                              let next: string[]
                                              if (!isSelected) {
                                                next = [...selected, src]
                                              } else if (selected.length <= 1) {
                                                return
                                              } else {
                                                next = selected.filter((s) => s !== src)
                                              }
                                              setMarketSourceDraft({
                                                ...marketSourceDraft,
                                                [m.code]: next,
                                              })
                                            }}
                                            onDragStart={(e) => {
                                              setDragState({ market: m.code, src })
                                              e.dataTransfer.effectAllowed = "move"
                                              e.dataTransfer.setData("text/plain", src)
                                            }}
                                            onDragOver={(e) => {
                                              if (
                                                dragState?.market === m.code &&
                                                dragState.src !== src &&
                                                isSelected
                                              ) {
                                                e.preventDefault()
                                                e.dataTransfer.dropEffect = "move"
                                                setDropTarget({ market: m.code, src })
                                              }
                                            }}
                                            onDragLeave={() => {
                                              if (
                                                dropTarget?.market === m.code &&
                                                dropTarget.src === src
                                              ) {
                                                setDropTarget(null)
                                              }
                                            }}
                                            onDrop={(e) => {
                                              e.preventDefault()
                                              if (
                                                dragState?.market === m.code &&
                                                dragState.src !== src &&
                                                isSelected
                                              ) {
                                                const fromIdx = selected.indexOf(dragState.src)
                                                const toIdx = selected.indexOf(src)
                                                const next = [...selected]
                                                next.splice(fromIdx, 1)
                                                next.splice(toIdx, 0, dragState.src)
                                                setMarketSourceDraft({
                                                  ...marketSourceDraft,
                                                  [m.code]: next,
                                                })
                                              }
                                              setDragState(null)
                                              setDropTarget(null)
                                            }}
                                            onDragEnd={() => {
                                              setDragState(null)
                                              setDropTarget(null)
                                            }}
                                            className={`px-2.5 py-1 text-xs rounded-full border transition-all ${
                                              isDragging
                                                ? "opacity-40 border-dashed border-[#ADB5BD]"
                                                : isDrop
                                                  ? "border-2 border-[#1A1A1A] bg-[#1A1A1A] text-white"
                                                  : isSelected
                                                    ? "bg-[#1A1A1A] text-white border-[#1A1A1A] cursor-grab active:cursor-grabbing"
                                                    : "bg-white text-[#6C757D] border-[#E9ECEF] hover:border-[#ADB5BD]"
                                            }`}
                                          >
                                            {marketSources.sourceNames[src] || src}
                                          </button>
                                        </div>
                                      )
                                    })}
                                  </div>
                                </div>
                              </div>
                            )
                          })}
                        </div>
                        {(testingSources || sourceTestResultsOrder.length > 0) && (
                          <div className="mt-4 border-t border-[#E9ECEF] pt-4">
                            <div className="flex items-center justify-between mb-2">
                              <label className="text-sm font-medium text-[#1A1A1A]">测试结果</label>
                              {testProgress && (
                                <span className="text-xs text-[#6C757D]">
                                  {testingSources
                                    ? `测试中 ${testProgress.tested}/${testProgress.total}`
                                    : `${testProgress.success}/${testProgress.total} 成功`}
                                </span>
                              )}
                            </div>
                            <div className="space-y-3">
                              {(() => {
                                const groups: Record<
                                  string,
                                  { key: string; src: string; result: SourceTestResult }[]
                                > = {}
                                for (const key of sourceTestResultsOrder) {
                                  const result = sourceTestResults[key]
                                  if (!result) continue
                                  const lastHyphen = key.lastIndexOf("-")
                                  const src = key.substring(0, lastHyphen)
                                  const mkt = key.substring(lastHyphen + 1)
                                  if (!groups[mkt]) groups[mkt] = []
                                  groups[mkt].push({ key, src, result })
                                }
                                return MARKET_OPTIONS.filter((m) => groups[m.code]).map((m) => (
                                  <div key={m.code}>
                                    <div className="text-xs font-medium text-[#495057] mb-1">
                                      {m.name}
                                    </div>
                                    <div className="space-y-1 w-full">
                                      {groups[m.code].map(({ key, src, result }) => (
                                        <div
                                          key={key}
                                          className="flex items-center gap-2 text-xs pl-2 w-full"
                                        >
                                          {result.success ? (
                                            <Check className="w-3 h-3 text-emerald-600 shrink-0" />
                                          ) : (
                                            <X className="w-3 h-3 text-red-500 shrink-0" />
                                          )}
                                          <span className="text-[#495057] w-20 shrink-0 truncate">
                                            {marketSources?.sourceNames[src] || src}
                                          </span>
                                          <span className="text-[#6C757D] w-24 shrink-0 truncate">
                                            {result.symbol || ""}
                                          </span>
                                          <span
                                            className={`w-24 shrink-0 truncate ${result.success ? "text-[#1A1A1A]" : "text-red-500"}`}
                                            title={result.success ? "" : result.error || "未知错误"}
                                          >
                                            {result.success
                                              ? result.rate
                                                ? ""
                                                : result.name || ""
                                              : result.error || "失败"}
                                          </span>
                                          <span
                                            className={`w-24 shrink-0 truncate ${result.success ? "text-[#1A1A1A]" : "text-red-500"}`}
                                          >
                                            {result.success
                                              ? result.rate
                                                ? `${result.rate}`
                                                : result.price
                                                  ? `${result.currency === "USD" ? "$" : result.currency === "CNY" ? "¥" : result.currency === "HKD" ? "HK$" : result.currency === "EUR" ? "€" : result.currency === "GBP" ? "£" : result.currency === "JPY" ? "¥" : ""}${result.price}`
                                                  : ""
                                              : ""}
                                          </span>
                                          {result.latency !== undefined && (
                                            <span className="text-[#6C757D] ml-auto shrink-0">
                                              {result.latency}ms
                                            </span>
                                          )}
                                        </div>
                                      ))}
                                    </div>
                                  </div>
                                ))
                              })()}
                            </div>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                )}

                {/* Display */}
                {activeSection === "display" && (
                  <div className="space-y-6">
                    <div>
                      <label className="block text-sm font-medium text-[#1A1A1A] mb-2">
                        涨跌配色
                      </label>
                      <p className="text-xs text-[#6C757D] mb-3">选择盈亏颜色显示方式。</p>
                      <div className="flex flex-wrap gap-2">
                        <button
                          onClick={() => setDraft({ ...draft, colorScheme: "green-up" })}
                          className={`flex items-center gap-1 px-3 py-1 text-xs rounded-full border transition-colors ${
                            draft.colorScheme === "green-up"
                              ? "bg-[#1A1A1A] text-white border-[#1A1A1A]"
                              : "bg-white text-[#6C757D] border-[#E9ECEF] hover:border-[#ADB5BD]"
                          }`}
                        >
                          <ArrowUp className="w-3 h-3 text-emerald-600" />
                          <ArrowDown className="w-3 h-3 text-orange-600" />
                        </button>
                        <button
                          onClick={() => setDraft({ ...draft, colorScheme: "red-up" })}
                          className={`flex items-center gap-1 px-3 py-1 text-xs rounded-full border transition-colors ${
                            draft.colorScheme === "red-up"
                              ? "bg-[#1A1A1A] text-white border-[#1A1A1A]"
                              : "bg-white text-[#6C757D] border-[#E9ECEF] hover:border-[#ADB5BD]"
                          }`}
                        >
                          <ArrowUp className="w-3 h-3 text-red-600" />
                          <ArrowDown className="w-3 h-3 text-emerald-600" />
                        </button>
                      </div>
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-[#1A1A1A] mb-2">
                        显示币种
                      </label>
                      <p className="text-xs text-[#6C757D] mb-3">
                        所有资产将按此币种汇总显示，汇率自动转换。
                      </p>
                      <div className="flex flex-wrap gap-2">
                        {DISPLAY_CURRENCIES.map((c) => (
                          <button
                            key={c.value}
                            onClick={() => setDraft({ ...draft, displayCurrency: c.value })}
                            className={`px-3 py-1 text-xs rounded-full border transition-colors ${
                              draft.displayCurrency === c.value
                                ? "bg-[#1A1A1A] text-white border-[#1A1A1A]"
                                : "bg-white text-[#6C757D] border-[#E9ECEF] hover:border-[#ADB5BD]"
                            }`}
                          >
                            {c.label}
                          </button>
                        ))}
                      </div>
                    </div>
                  </div>
                )}

                {/* Notify */}
                {activeSection === "notify" && (
                  <div>
                    <div className="flex items-center justify-between mb-4">
                      <div>
                        <label className="block text-sm font-medium text-[#1A1A1A]">
                          Telegram 通知
                        </label>
                        <p className="text-xs text-[#6C757D] mt-1">
                          通过 Telegram Bot 接收投资组合管理通知
                        </p>
                      </div>
                      <button
                        onClick={() =>
                          setDraft({ ...draft, telegramEnabled: !draft.telegramEnabled })
                        }
                        className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                          draft.telegramEnabled ? "bg-[#1A1A1A]" : "bg-[#E9ECEF]"
                        }`}
                      >
                        <span
                          className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                            draft.telegramEnabled ? "translate-x-6" : "translate-x-1"
                          }`}
                        />
                      </button>
                    </div>

                    {draft.telegramEnabled && (
                      <div className="space-y-4 mt-4">
                        <div>
                          <label className="block text-xs font-medium text-[#6C757D] mb-1">
                            Bot Token
                          </label>
                          <input
                            type="password"
                            value={draft.telegramBotToken}
                            onChange={(e) =>
                              setDraft({ ...draft, telegramBotToken: e.target.value })
                            }
                            placeholder="从 @BotFather 获取"
                            className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                          />
                        </div>

                        <div>
                          <label className="block text-xs font-medium text-[#6C757D] mb-1">
                            Chat ID
                          </label>
                          <input
                            type="text"
                            value={draft.telegramChatID}
                            onChange={(e) => setDraft({ ...draft, telegramChatID: e.target.value })}
                            placeholder="发送 /start 给 @userinfobot 获取"
                            className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                          />
                        </div>

                        <div className="flex items-center gap-2">
                          <select
                            value={testType}
                            onChange={(e) => setTestType(e.target.value as typeof testType)}
                            className="px-2 py-1.5 text-xs border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
                          >
                            <option value="connection">测试连接</option>
                            <option value="price">价格波动告警</option>
                            <option value="drift">配比偏离提醒</option>
                            <option value="summary">组合摘要</option>
                          </select>
                          <button
                            onClick={() =>
                              testType === "connection"
                                ? handleTestConnection()
                                : handleTestMessage(testType)
                            }
                            disabled={testing}
                            className="px-3 py-1.5 text-xs text-[#1A1A1A] border border-[#E9ECEF] rounded-lg hover:bg-[#F1F3F5] transition-colors disabled:opacity-50"
                          >
                            {testing ? "发送中..." : "发送测试"}
                          </button>
                        </div>
                        {testResult && (
                          <span
                            className={`text-xs ${
                              testResult.success ? "text-green-600" : "text-red-500"
                            }`}
                          >
                            {testResult.message}
                          </span>
                        )}

                        <div className="space-y-3 pt-2">
                          <label className="block text-xs font-medium text-[#6C757D]">
                            通知类型
                          </label>

                          <div className="flex items-center justify-between">
                            <div>
                              <span className="text-sm text-[#1A1A1A]">价格大幅波动</span>
                              <div className="flex items-center gap-2 mt-1">
                                <span className="text-xs text-[#6C757D]">阈值:</span>
                                <input
                                  type="number"
                                  min="1"
                                  max="50"
                                  value={draft.telegramPriceThreshold}
                                  onChange={(e) =>
                                    setDraft({
                                      ...draft,
                                      telegramPriceThreshold: Math.max(
                                        1,
                                        Math.min(50, Number(e.target.value) || 1)
                                      ),
                                    })
                                  }
                                  className="w-12 px-2 py-1 text-xs text-center border border-[#E9ECEF] rounded focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
                                />
                                <span className="text-xs text-[#6C757D]">%</span>
                              </div>
                            </div>
                            <button
                              onClick={() =>
                                setDraft({
                                  ...draft,
                                  telegramPriceAlert: !draft.telegramPriceAlert,
                                })
                              }
                              className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
                                draft.telegramPriceAlert ? "bg-[#1A1A1A]" : "bg-[#E9ECEF]"
                              }`}
                            >
                              <span
                                className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform ${
                                  draft.telegramPriceAlert ? "translate-x-4.5" : "translate-x-0.5"
                                }`}
                              />
                            </button>
                          </div>

                          <div className="flex items-center justify-between">
                            <span className="text-sm text-[#1A1A1A]">配比偏离提醒</span>
                            <button
                              onClick={() =>
                                setDraft({
                                  ...draft,
                                  telegramDriftAlert: !draft.telegramDriftAlert,
                                })
                              }
                              className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
                                draft.telegramDriftAlert ? "bg-[#1A1A1A]" : "bg-[#E9ECEF]"
                              }`}
                            >
                              <span
                                className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform ${
                                  draft.telegramDriftAlert ? "translate-x-4.5" : "translate-x-0.5"
                                }`}
                              />
                            </button>
                          </div>

                          <div className="flex items-center justify-between">
                            <span className="text-sm text-[#1A1A1A]">定期组合摘要</span>
                            <select
                              value={draft.telegramSummaryInterval}
                              onChange={(e) =>
                                setDraft({ ...draft, telegramSummaryInterval: e.target.value })
                              }
                              className="px-2 py-1 text-xs border border-[#E9ECEF] rounded focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
                            >
                              {SUMMARY_INTERVALS.map((opt) => (
                                <option key={opt.value} value={opt.value}>
                                  {opt.label}
                                </option>
                              ))}
                            </select>
                          </div>
                        </div>
                      </div>
                    )}

                    {/* Bark 通知 */}
                    <div className="border-t border-[#E9ECEF] pt-4 mt-4">
                      <div className="flex items-center justify-between mb-4">
                        <div>
                          <label className="block text-sm font-medium text-[#1A1A1A]">
                            Bark 通知
                          </label>
                          <p className="text-xs text-[#6C757D] mt-1">
                            通过 Bark iOS App 接收推送通知
                          </p>
                        </div>
                        <button
                          onClick={() => setDraft({ ...draft, barkEnabled: !draft.barkEnabled })}
                          className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                            draft.barkEnabled ? "bg-[#1A1A1A]" : "bg-[#E9ECEF]"
                          }`}
                        >
                          <span
                            className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                              draft.barkEnabled ? "translate-x-6" : "translate-x-1"
                            }`}
                          />
                        </button>
                      </div>

                      {draft.barkEnabled && (
                        <div className="space-y-4 mt-4">
                          <div>
                            <label className="block text-xs font-medium text-[#6C757D] mb-1">
                              Device Key
                            </label>
                            <input
                              type="password"
                              value={draft.barkDeviceKey}
                              onChange={(e) =>
                                setDraft({ ...draft, barkDeviceKey: e.target.value })
                              }
                              placeholder="从 Bark iOS App 获取"
                              className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                            />
                          </div>

                          <div>
                            <label className="block text-xs font-medium text-[#6C757D] mb-1">
                              Server URL
                            </label>
                            <input
                              type="text"
                              value={draft.barkServerURL}
                              onChange={(e) =>
                                setDraft({ ...draft, barkServerURL: e.target.value })
                              }
                              placeholder="默认 https://api.day.app，可自建"
                              className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                            />
                          </div>

                          <div className="flex items-center gap-2">
                            <select
                              value={barkTestType}
                              onChange={(e) =>
                                setBarkTestType(e.target.value as typeof barkTestType)
                              }
                              className="px-2 py-1.5 text-xs border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
                            >
                              <option value="connection">测试连接</option>
                              <option value="price">价格波动告警</option>
                              <option value="drift">配比偏离提醒</option>
                              <option value="summary">组合摘要</option>
                            </select>
                            <button
                              onClick={() =>
                                barkTestType === "connection"
                                  ? handleBarkTestConnection()
                                  : handleBarkTestMessage(barkTestType)
                              }
                              disabled={barkTesting}
                              className="px-3 py-1.5 text-xs text-[#1A1A1A] border border-[#E9ECEF] rounded-lg hover:bg-[#F1F3F5] transition-colors disabled:opacity-50"
                            >
                              {barkTesting ? "发送中..." : "发送测试"}
                            </button>
                          </div>
                          {barkTestResult && (
                            <span
                              className={`text-xs ${
                                barkTestResult.success ? "text-green-600" : "text-red-500"
                              }`}
                            >
                              {barkTestResult.message}
                            </span>
                          )}

                          <div className="space-y-3 pt-2">
                            <label className="block text-xs font-medium text-[#6C757D]">
                              通知类型
                            </label>

                            <div className="flex items-center justify-between">
                              <div>
                                <span className="text-sm text-[#1A1A1A]">价格大幅波动</span>
                                <div className="flex items-center gap-2 mt-1">
                                  <span className="text-xs text-[#6C757D]">阈值:</span>
                                  <input
                                    type="number"
                                    min="1"
                                    max="50"
                                    value={draft.barkPriceThreshold}
                                    onChange={(e) =>
                                      setDraft({
                                        ...draft,
                                        barkPriceThreshold: Math.max(
                                          1,
                                          Math.min(50, Number(e.target.value) || 1)
                                        ),
                                      })
                                    }
                                    className="w-12 px-2 py-1 text-xs text-center border border-[#E9ECEF] rounded focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
                                  />
                                  <span className="text-xs text-[#6C757D]">%</span>
                                </div>
                              </div>
                              <button
                                onClick={() =>
                                  setDraft({
                                    ...draft,
                                    barkPriceAlert: !draft.barkPriceAlert,
                                  })
                                }
                                className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
                                  draft.barkPriceAlert ? "bg-[#1A1A1A]" : "bg-[#E9ECEF]"
                                }`}
                              >
                                <span
                                  className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform ${
                                    draft.barkPriceAlert ? "translate-x-4.5" : "translate-x-0.5"
                                  }`}
                                />
                              </button>
                            </div>

                            <div className="flex items-center justify-between">
                              <span className="text-sm text-[#1A1A1A]">配比偏离提醒</span>
                              <button
                                onClick={() =>
                                  setDraft({
                                    ...draft,
                                    barkDriftAlert: !draft.barkDriftAlert,
                                  })
                                }
                                className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
                                  draft.barkDriftAlert ? "bg-[#1A1A1A]" : "bg-[#E9ECEF]"
                                }`}
                              >
                                <span
                                  className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform ${
                                    draft.barkDriftAlert ? "translate-x-4.5" : "translate-x-0.5"
                                  }`}
                                />
                              </button>
                            </div>

                            <div className="flex items-center justify-between">
                              <span className="text-sm text-[#1A1A1A]">定期组合摘要</span>
                              <select
                                value={draft.barkSummaryInterval}
                                onChange={(e) =>
                                  setDraft({ ...draft, barkSummaryInterval: e.target.value })
                                }
                                className="px-2 py-1 text-xs border border-[#E9ECEF] rounded focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
                              >
                                {SUMMARY_INTERVALS.map((opt) => (
                                  <option key={opt.value} value={opt.value}>
                                    {opt.label}
                                  </option>
                                ))}
                              </select>
                            </div>
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
                )}

                {/* User security */}
                {activeSection === "security" && (
                  <div className="space-y-6">
                    <PasskeyManager />
                  </div>
                )}

                {/* System security */}
                {activeSection === "system-security" && (
                  <div className="space-y-6">
                    {/* OIDC */}
                    {userRole === "admin" && (
                      <div>
                        <div className="flex items-center justify-between mb-4">
                          <div>
                            <label className="block text-sm font-medium text-[#1A1A1A]">
                              SSO 登录 (OIDC)
                            </label>
                            <p className="text-xs text-[#6C757D] mt-1">
                              配置 OpenID Connect 单点登录
                            </p>
                          </div>
                          <button
                            onClick={() =>
                              setOidcDraft({ ...oidcDraft, enabled: !oidcDraft.enabled })
                            }
                            className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                              oidcDraft.enabled ? "bg-[#1A1A1A]" : "bg-[#E9ECEF]"
                            }`}
                          >
                            <span
                              className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                                oidcDraft.enabled ? "translate-x-6" : "translate-x-1"
                              }`}
                            />
                          </button>
                        </div>

                        {oidcDraft.enabled && (
                          <div className="space-y-4">
                            <div>
                              <label className="block text-xs font-medium text-[#6C757D] mb-1">
                                Issuer URL
                              </label>
                              <input
                                type="text"
                                value={oidcDraft.issuer}
                                onChange={(e) =>
                                  setOidcDraft({ ...oidcDraft, issuer: e.target.value })
                                }
                                placeholder="https://your-provider.example.com"
                                className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                              />
                            </div>

                            <div>
                              <label className="block text-xs font-medium text-[#6C757D] mb-1">
                                Client ID
                              </label>
                              <input
                                type="text"
                                value={oidcDraft.clientID}
                                onChange={(e) =>
                                  setOidcDraft({ ...oidcDraft, clientID: e.target.value })
                                }
                                className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                              />
                            </div>

                            <div>
                              <label className="block text-xs font-medium text-[#6C757D] mb-1">
                                Client Secret
                              </label>
                              <input
                                type="password"
                                value={oidcDraft.clientSecret}
                                onChange={(e) =>
                                  setOidcDraft({ ...oidcDraft, clientSecret: e.target.value })
                                }
                                className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                              />
                            </div>

                            <div>
                              <label className="block text-xs font-medium text-[#6C757D] mb-1">
                                Redirect URL
                              </label>
                              <input
                                type="text"
                                value={oidcDraft.redirectURL}
                                onChange={(e) =>
                                  setOidcDraft({ ...oidcDraft, redirectURL: e.target.value })
                                }
                                placeholder="http://localhost:3000/api/auth/oidc/callback"
                                className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                              />
                            </div>
                          </div>
                        )}
                      </div>
                    )}

                    {/* WebAuthn */}
                    {userRole === "admin" && (
                      <WebAuthnConfigSection draft={webauthnDraft} onChange={setWebauthnDraft} />
                    )}
                  </div>
                )}
              </div>
            </div>

            {/* Fixed Footer */}
            <div className="flex justify-end gap-3 px-6 py-4 border-t border-[#E9ECEF]">
              <button
                onClick={() => setIsOpen(false)}
                className="px-4 py-2 text-sm text-[#6C757D] hover:text-[#1A1A1A] transition-colors"
              >
                取消
              </button>
              <button
                onClick={handleSave}
                disabled={saving}
                className="px-4 py-2 text-sm bg-[#1A1A1A] text-white rounded-lg hover:bg-[#333] transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
              >
                {saving ? "保存中..." : "保存"}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}

function PasskeyManager() {
  const [credentials, setCredentials] = useState<api.WebAuthnCredentialInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [registering, setRegistering] = useState(false)
  const [newName, setNewName] = useState("")
  const [error, setError] = useState("")

  const loadCredentials = async () => {
    try {
      const creds = await api.webAuthnListCredentials()
      setCredentials(creds)
    } catch (e) {
      console.error("Failed to load credentials", e)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      try {
        const creds = await api.webAuthnListCredentials()
        if (!cancelled) setCredentials(creds)
      } catch (e) {
        console.error("Failed to load credentials", e)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => {
      cancelled = true
    }
  }, [])

  const handleRegister = async () => {
    setRegistering(true)
    setError("")
    try {
      const options = await api.webAuthnRegisterStart(newName)
      const { startRegistration } = await import("@simplewebauthn/browser")
      const credential = await startRegistration({ optionsJSON: options })
      await api.webAuthnRegisterFinish(credential)
      setNewName("")
      loadCredentials()
    } catch (e) {
      setError(e instanceof Error ? e.message : "注册Passkey失败")
    } finally {
      setRegistering(false)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await api.webAuthnDeleteCredential(id)
      loadCredentials()
    } catch (e) {
      console.error("Failed to delete credential", e)
    }
  }

  if (loading) return null

  return (
    <div>
      <label className="block text-sm font-medium text-[#1A1A1A] mb-1">Passkey 管理</label>
      <p className="text-xs text-[#6C757D] mb-4">管理已注册的 Passkey 凭证</p>

      {credentials.length > 0 && (
        <div className="space-y-2 mb-4">
          {credentials.map((cred) => (
            <div
              key={cred.id}
              className="flex items-center justify-between py-2 px-3 bg-[#F8F9FA] rounded-lg"
            >
              <div>
                <span className="text-sm text-[#1A1A1A]">{cred.name || "未命名"}</span>
                <span className="text-xs text-[#6C757D] ml-2">
                  {cred.lastUsedAt
                    ? `上次使用: ${new Date(cred.lastUsedAt * 1000).toLocaleDateString()}`
                    : "未使用"}
                </span>
              </div>
              <button
                onClick={() => handleDelete(cred.id)}
                className="text-xs text-red-500 hover:text-red-700"
              >
                删除
              </button>
            </div>
          ))}
        </div>
      )}

      <div className="flex items-center gap-2">
        <input
          type="text"
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          placeholder="Passkey 名称（可选）"
          className="flex-1 px-3 py-1.5 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
        />
        <button
          onClick={handleRegister}
          disabled={registering}
          className="px-3 py-1.5 text-xs text-[#1A1A1A] border border-[#E9ECEF] rounded-lg hover:bg-[#F1F3F5] transition-colors disabled:opacity-50"
        >
          {registering ? "注册中..." : "添加 Passkey"}
        </button>
      </div>
      {error && <p className="text-xs text-red-500 mt-2">{error}</p>}
    </div>
  )
}

function WebAuthnConfigSection({
  draft,
  onChange,
}: {
  draft: { enabled: boolean; rpid: string; rpOrigins: string }
  onChange: (d: { enabled: boolean; rpid: string; rpOrigins: string }) => void
}) {
  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div>
          <label className="block text-sm font-medium text-[#1A1A1A]">
            Passkey 登录 (WebAuthn)
          </label>
          <p className="text-xs text-[#6C757D] mt-1">配置 Passkey 无密码登录</p>
        </div>
        <button
          onClick={() => onChange({ ...draft, enabled: !draft.enabled })}
          className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
            draft.enabled ? "bg-[#1A1A1A]" : "bg-[#E9ECEF]"
          }`}
        >
          <span
            className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
              draft.enabled ? "translate-x-6" : "translate-x-1"
            }`}
          />
        </button>
      </div>

      {draft.enabled && (
        <div className="space-y-3">
          <div>
            <label className="block text-xs font-medium text-[#6C757D] mb-1">RPID (域名)</label>
            <input
              type="text"
              value={draft.rpid}
              onChange={(e) => onChange({ ...draft, rpid: e.target.value })}
              placeholder="localhost"
              className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-[#6C757D] mb-1">
              RPOrigins (逗号分隔)
            </label>
            <input
              type="text"
              value={draft.rpOrigins}
              onChange={(e) => onChange({ ...draft, rpOrigins: e.target.value })}
              placeholder="http://localhost:3000"
              className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
            />
          </div>
        </div>
      )}
    </div>
  )
}

const ASSET_ORDER: AssetId[] = ["stocks", "bonds", "cash", "commodities"]
const ASSET_KEYS = ASSET_ORDER.map(
  (id) => `target${id.charAt(0).toUpperCase() + id.slice(1)}` as keyof Settings
)
const MIN_SEG = 1

function TargetAllocationBar({
  draft,
  onChange,
}: {
  draft: Settings
  onChange: (patch: Partial<Settings>) => void
}) {
  const barRef = useRef<HTMLDivElement>(null)
  const [dragIdx, setDragIdx] = useState<number | null>(null)
  const dragRef = useRef<{
    idx: number
    startX: number
    perPx: number
    belowSum: number
    aboveSum: number
    currentPcts: number[]
  } | null>(null)
  const currentPcts = ASSET_KEYS.map((k) => draft[k] as number)

  const handlePointerDown = useCallback(
    (idx: number, e: React.PointerEvent) => {
      const bar = barRef.current
      if (!bar) return
      const w = bar.getBoundingClientRect().width
      if (w === 0) return

      const pcts = currentPcts
      const belowSum = pcts.slice(0, idx).reduce((s, v) => s + v, 0)
      const aboveSum = pcts.slice(idx + 2).reduce((s, v) => s + v, 0)

      dragRef.current = {
        idx,
        startX: e.clientX,
        perPx: 100 / w,
        belowSum,
        aboveSum,
        currentPcts: [...pcts],
      }
      setDragIdx(idx)
      ;(e.target as HTMLElement).setPointerCapture(e.pointerId)
    },
    [currentPcts]
  )

  useEffect(() => {
    if (dragIdx === null) return

    const onMove = (e: PointerEvent) => {
      const d = dragRef.current
      if (!d) return
      const delta = (e.clientX - d.startX) * d.perPx
      const lo = MIN_SEG
      const hi = 100 - d.belowSum - d.aboveSum - MIN_SEG
      const newLeft = Math.round(Math.max(lo, Math.min(hi, d.currentPcts[d.idx] + delta)))
      const newRight = 100 - d.belowSum - d.aboveSum - newLeft

      const patch: Record<string, number> = {}
      patch[ASSET_KEYS[d.idx]] = newLeft
      patch[ASSET_KEYS[d.idx + 1]] = newRight
      onChange(patch as Partial<Settings>)
    }

    const onUp = () => {
      dragRef.current = null
      setDragIdx(null)
    }

    window.addEventListener("pointermove", onMove)
    window.addEventListener("pointerup", onUp)
    return () => {
      window.removeEventListener("pointermove", onMove)
      window.removeEventListener("pointerup", onUp)
    }
  }, [dragIdx, onChange])

  return (
    <div>
      <div ref={barRef} className="relative flex h-10 rounded-lg overflow-hidden select-none">
        {ASSET_ORDER.map((id, i) => {
          const def = ASSET_DEFINITIONS[id]
          const pct = currentPcts[i]
          return (
            <div
              key={id}
              className="relative flex items-center justify-center transition-[width] duration-75"
              style={{ width: `${pct}%`, backgroundColor: def.color }}
            >
              {pct >= 10 && (
                <span
                  className={`text-[11px] font-medium leading-none whitespace-nowrap ${id === "cash" ? "text-[#495057]" : "text-white"}`}
                >
                  {def.name} {pct}%
                </span>
              )}
              {i < 3 && (
                <div
                  className="absolute right-0 top-0 bottom-0 w-1.5 z-10 flex items-center justify-center cursor-col-resize touch-none"
                  onPointerDown={(e) => handlePointerDown(i, e)}
                >
                  <div
                    className={`w-0.5 h-5 rounded-full transition-colors ${
                      dragIdx === i ? "bg-white" : "bg-white/50"
                    }`}
                  />
                </div>
              )}
            </div>
          )
        })}
      </div>
      <div className="flex items-center gap-4 mt-2">
        {ASSET_ORDER.map((id, i) => {
          const def = ASSET_DEFINITIONS[id]
          const pct = currentPcts[i]
          return (
            <div key={id} className="flex items-center gap-1.5">
              <div
                className={`w-2 h-2 rounded-full shrink-0 ${id === "cash" ? "border border-[#ADB5BD]" : ""}`}
                style={{ backgroundColor: def.color }}
              />
              <span className="text-[11px] text-[#6C757D]">
                {def.name}
                {pct < 10 ? ` ${pct}%` : ""}
              </span>
            </div>
          )
        })}
      </div>
    </div>
  )
}
