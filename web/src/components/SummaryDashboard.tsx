import { useEffect, useMemo } from "react"
import type { ReactNode } from "react"
import {
  AlertCircle,
  ArrowDownRight,
  ArrowUpRight,
  Layers3,
  LoaderCircle,
  PieChart,
  RefreshCw,
  WalletCards,
  X,
} from "lucide-react"
import { PortfolioSummary, ASSET_DEFINITIONS, AssetId, ColorScheme } from "../types"
import { formatCurrencyByCode, toDecimal } from "../utils"

interface Props {
  summary: PortfolioSummary | null
  colorScheme: ColorScheme
  displayCurrency: string
  loading?: boolean
  error?: string
  onRetry?: () => void
  onClose: () => void
}

const ASSET_IDS: AssetId[] = ["stocks", "bonds", "cash", "commodities"]

function getPerformance(total: string, principal: string) {
  const totalD = toDecimal(total)
  const principalD = toDecimal(principal)
  const profit = totalD.minus(principalD)
  if (!principalD.isPositive()) return { profit, rate: null }
  const rate = profit.div(principalD).times(100).toNumber()
  return { profit, rate: Number.isFinite(rate) ? rate : null }
}

function getPercent(value: string, total: string) {
  const totalD = toDecimal(total)
  if (!totalD.isPositive()) return 0
  const percent = toDecimal(value).div(totalD).times(100).toNumber()
  return Number.isFinite(percent) ? Math.max(0, percent) : 0
}

export default function SummaryDashboard({
  summary,
  colorScheme,
  displayCurrency,
  loading = false,
  error = "",
  onRetry,
  onClose,
}: Props) {
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose()
    }
    document.addEventListener("keydown", handleKeyDown)
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = "hidden"
    return () => {
      document.removeEventListener("keydown", handleKeyDown)
      document.body.style.overflow = previousOverflow
    }
  }, [onClose, summary])

  const sortedPortfolios = useMemo(
    () =>
      [...(summary?.portfolios ?? [])].sort((a, b) =>
        toDecimal(b.total).comparedTo(toDecimal(a.total))
      ),
    [summary]
  )

  if (!summary) {
    return (
      <DialogFrame onClose={onClose} subtitle={`统一折算为 ${displayCurrency}`}>
        <div className="flex min-h-80 flex-col items-center justify-center px-6 text-center">
          {loading ? (
            <>
              <LoaderCircle size={30} className="animate-spin text-[#6C757D]" />
              <p className="mt-4 text-sm font-medium text-[#343A40]">正在汇总全部投资组合</p>
              <p className="mt-1.5 text-xs text-[#ADB5BD]">正在折算资产、投入与收益数据…</p>
            </>
          ) : (
            <>
              <span className="flex size-12 items-center justify-center rounded-full bg-red-50 text-red-500">
                <AlertCircle size={23} />
              </span>
              <p className="mt-4 text-sm font-medium text-[#343A40]">无法加载组合汇总</p>
              <p className="mt-1.5 max-w-sm text-xs leading-5 text-[#868E96]">
                {error || "请检查网络连接后重试"}
              </p>
              {onRetry && (
                <button
                  type="button"
                  onClick={onRetry}
                  className="mt-5 inline-flex items-center gap-2 rounded-lg bg-[#1A1A1A] px-4 py-2 text-xs font-medium text-white transition-colors hover:bg-[#343A40]"
                >
                  <RefreshCw size={14} />
                  重新加载
                </button>
              )}
            </>
          )}
        </div>
      </DialogFrame>
    )
  }

  const greenUp = colorScheme === "green-up"
  const totalPerformance = getPerformance(summary.total, summary.principal)
  const positiveClass = greenUp ? "text-emerald-600" : "text-red-600"
  const negativeClass = greenUp ? "text-red-600" : "text-emerald-600"
  const performanceClass = (value: number) => {
    if (value > 0) return positiveClass
    if (value < 0) return negativeClass
    return "text-[#6C757D]"
  }

  let angle = 0
  const gradientStops = ASSET_IDS.flatMap((id) => {
    const percent = getPercent(summary.assets[id] || "0", summary.total)
    const start = angle
    angle += percent * 3.6
    return percent > 0
      ? [`${ASSET_DEFINITIONS[id].color} ${start}deg`, `${ASSET_DEFINITIONS[id].color} ${angle}deg`]
      : []
  })
  const allocationGradient = gradientStops.length
    ? `conic-gradient(${gradientStops.join(", ")})`
    : "#F1F3F5"

  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-[#111827]/45 p-0 backdrop-blur-[2px] sm:items-center sm:p-6"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose()
      }}
    >
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby="portfolio-summary-title"
        className="flex max-h-[94dvh] w-full max-w-6xl flex-col overflow-hidden rounded-t-2xl bg-[#F8F9FA] shadow-2xl sm:max-h-[88vh] sm:rounded-2xl"
      >
        <header className="flex shrink-0 items-center justify-between border-b border-[#E9ECEF] bg-white px-5 py-4 sm:px-7">
          <div className="flex items-center gap-3">
            <div className="flex size-10 items-center justify-center rounded-xl bg-[#1A1A1A] text-white">
              <Layers3 size={19} strokeWidth={1.8} />
            </div>
            <div>
              <h2
                id="portfolio-summary-title"
                className="text-base font-semibold text-[#1A1A1A] sm:text-lg"
              >
                投资组合汇总
              </h2>
              <p className="mt-0.5 text-xs text-[#868E96]">
                {summary.portfolios.length} 个组合 · 统一折算为 {displayCurrency}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="关闭投资组合汇总"
            className="flex size-9 items-center justify-center rounded-lg text-[#6C757D] transition-colors hover:bg-[#F1F3F5] hover:text-[#1A1A1A] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#1A1A1A]"
          >
            <X size={20} />
          </button>
        </header>

        <div className="overflow-y-auto p-4 sm:p-7">
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <MetricCard
              label="总资产"
              value={formatCurrencyByCode(summary.total, displayCurrency)}
              hint="当前全部组合市值"
              icon={<WalletCards size={18} />}
              featured
            />
            <MetricCard
              label="累计投入"
              value={formatCurrencyByCode(summary.principal, displayCurrency)}
              hint="净转入本金"
            />
            <MetricCard
              label="累计收益"
              value={formatCurrencyByCode(totalPerformance.profit.toString(), displayCurrency)}
              hint="总资产减累计投入"
              valueClass={performanceClass(totalPerformance.profit.comparedTo(0))}
            />
            <MetricCard
              label="收益率"
              value={
                totalPerformance.rate === null
                  ? "—"
                  : `${totalPerformance.rate > 0 ? "+" : ""}${totalPerformance.rate.toFixed(2)}%`
              }
              hint={totalPerformance.rate === null ? "暂无有效投入" : "基于累计投入计算"}
              valueClass={performanceClass(totalPerformance.rate ?? 0)}
              icon={
                totalPerformance.profit.isNegative() ? (
                  <ArrowDownRight size={18} />
                ) : (
                  <ArrowUpRight size={18} />
                )
              }
            />
          </div>

          <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1.05fr)_minmax(0,1.45fr)]">
            <section className="rounded-xl border border-[#E9ECEF] bg-white p-5 sm:p-6">
              <div className="flex items-center gap-2">
                <PieChart size={17} className="text-[#6C757D]" />
                <h3 className="text-sm font-semibold text-[#1A1A1A]">资产配置</h3>
              </div>

              <div className="mt-6 flex flex-col items-center gap-7 sm:flex-row sm:items-center">
                <div
                  className="relative size-42 shrink-0 rounded-full"
                  style={{ background: allocationGradient }}
                  aria-label="资产配置环形图"
                >
                  <div className="absolute inset-7 flex flex-col items-center justify-center rounded-full bg-white shadow-[inset_0_0_0_1px_#F1F3F5]">
                    <span className="text-[11px] text-[#868E96]">总资产</span>
                    <span className="mt-1 max-w-27 truncate font-mono text-sm font-semibold text-[#1A1A1A]">
                      {formatCurrencyByCode(summary.total, displayCurrency)}
                    </span>
                  </div>
                </div>

                <div className="grid w-full grid-cols-2 gap-x-5 gap-y-4">
                  {ASSET_IDS.map((id) => {
                    const value = summary.assets[id] || "0"
                    const percent = getPercent(value, summary.total)
                    return (
                      <div key={id} className="min-w-0">
                        <div className="flex items-center gap-2">
                          <span
                            className="size-2.5 shrink-0 rounded-full ring-1 ring-black/5"
                            style={{ backgroundColor: ASSET_DEFINITIONS[id].color }}
                          />
                          <span className="truncate text-xs text-[#6C757D]">
                            {ASSET_DEFINITIONS[id].name}
                          </span>
                        </div>
                        <p className="mt-1.5 font-mono text-base font-semibold text-[#1A1A1A]">
                          {percent.toFixed(1)}%
                        </p>
                        <p className="mt-0.5 truncate text-[11px] text-[#ADB5BD]">
                          {formatCurrencyByCode(value, displayCurrency)}
                        </p>
                      </div>
                    )
                  })}
                </div>
              </div>
            </section>

            <section className="overflow-hidden rounded-xl border border-[#E9ECEF] bg-white">
              <div className="flex items-center justify-between border-b border-[#F1F3F5] px-5 py-4 sm:px-6">
                <div>
                  <h3 className="text-sm font-semibold text-[#1A1A1A]">组合表现</h3>
                  <p className="mt-0.5 text-[11px] text-[#868E96]">按当前资产从高到低排列</p>
                </div>
                <span className="rounded-full bg-[#F1F3F5] px-2.5 py-1 text-[11px] text-[#6C757D]">
                  {sortedPortfolios.length} 个组合
                </span>
              </div>

              {sortedPortfolios.length === 0 ? (
                <div className="flex min-h-56 flex-col items-center justify-center px-6 text-center">
                  <Layers3 size={28} strokeWidth={1.4} className="text-[#ADB5BD]" />
                  <p className="mt-3 text-sm font-medium text-[#495057]">暂无投资组合</p>
                  <p className="mt-1 text-xs text-[#ADB5BD]">创建组合后，其表现会显示在这里</p>
                </div>
              ) : (
                <div className="divide-y divide-[#F1F3F5]">
                  {sortedPortfolios.map((portfolio, index) => {
                    const performance = getPerformance(portfolio.total, portfolio.principal)
                    const share = getPercent(portfolio.total, summary.total)
                    return (
                      <article
                        key={portfolio.id}
                        className="px-5 py-4 transition-colors hover:bg-[#FCFCFD] sm:px-6"
                      >
                        <div className="flex items-start gap-3">
                          <span className="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-lg bg-[#F1F3F5] font-mono text-[11px] font-semibold text-[#6C757D]">
                            {String(index + 1).padStart(2, "0")}
                          </span>
                          <div className="min-w-0 flex-1">
                            <div className="flex items-start justify-between gap-4">
                              <div className="min-w-0">
                                <h4 className="truncate text-sm font-medium text-[#212529]">
                                  {portfolio.name}
                                </h4>
                                <p className="mt-1 text-[11px] text-[#868E96]">
                                  占总资产 {share.toFixed(1)}%
                                </p>
                              </div>
                              <div className="shrink-0 text-right">
                                <p className="font-mono text-sm font-semibold text-[#212529]">
                                  {formatCurrencyByCode(portfolio.total, displayCurrency)}
                                </p>
                                <p
                                  className={`mt-1 text-xs font-medium ${performanceClass(performance.rate ?? 0)}`}
                                >
                                  {performance.rate === null
                                    ? "—"
                                    : `${performance.rate > 0 ? "+" : ""}${performance.rate.toFixed(2)}%`}
                                </p>
                              </div>
                            </div>

                            <div className="mt-3 flex h-1.5 overflow-hidden rounded-full bg-[#F1F3F5]">
                              {ASSET_IDS.map((id) => {
                                const percent = getPercent(
                                  portfolio.assets[id] || "0",
                                  portfolio.total
                                )
                                return (
                                  <span
                                    key={id}
                                    style={{
                                      width: `${percent}%`,
                                      backgroundColor: ASSET_DEFINITIONS[id].color,
                                    }}
                                  />
                                )
                              })}
                            </div>
                            <div className="mt-2 flex items-center justify-between gap-3 text-[11px] text-[#868E96]">
                              <span>
                                投入 {formatCurrencyByCode(portfolio.principal, displayCurrency)}
                              </span>
                              <span className={performanceClass(performance.profit.comparedTo(0))}>
                                收益{" "}
                                {formatCurrencyByCode(
                                  performance.profit.toString(),
                                  displayCurrency
                                )}
                              </span>
                            </div>
                          </div>
                        </div>
                      </article>
                    )
                  })}
                </div>
              )}
            </section>
          </div>
        </div>
      </section>
    </div>
  )
}

function DialogFrame({
  children,
  subtitle,
  onClose,
}: {
  children: ReactNode
  subtitle: string
  onClose: () => void
}) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-[#111827]/45 p-0 backdrop-blur-[2px] sm:items-center sm:p-6"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose()
      }}
    >
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby="portfolio-summary-title"
        className="flex max-h-[94dvh] w-full max-w-6xl flex-col overflow-hidden rounded-t-2xl bg-[#F8F9FA] shadow-2xl sm:max-h-[88vh] sm:rounded-2xl"
      >
        <header className="flex shrink-0 items-center justify-between border-b border-[#E9ECEF] bg-white px-5 py-4 sm:px-7">
          <div className="flex items-center gap-3">
            <div className="flex size-10 items-center justify-center rounded-xl bg-[#1A1A1A] text-white">
              <Layers3 size={19} strokeWidth={1.8} />
            </div>
            <div>
              <h2
                id="portfolio-summary-title"
                className="text-base font-semibold text-[#1A1A1A] sm:text-lg"
              >
                投资组合汇总
              </h2>
              <p className="mt-0.5 text-xs text-[#868E96]">{subtitle}</p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="关闭投资组合汇总"
            className="flex size-9 items-center justify-center rounded-lg text-[#6C757D] transition-colors hover:bg-[#F1F3F5] hover:text-[#1A1A1A] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#1A1A1A]"
          >
            <X size={20} />
          </button>
        </header>
        {children}
      </section>
    </div>
  )
}

function MetricCard({
  label,
  value,
  hint,
  icon,
  featured = false,
  valueClass = "text-[#1A1A1A]",
}: {
  label: string
  value: string
  hint: string
  icon?: ReactNode
  featured?: boolean
  valueClass?: string
}) {
  return (
    <div
      className={`min-w-0 rounded-xl border p-4 sm:p-5 ${
        featured ? "border-[#1A1A1A] bg-[#1A1A1A] text-white" : "border-[#E9ECEF] bg-white"
      }`}
    >
      <div className="flex items-center justify-between gap-2">
        <p className={`text-xs ${featured ? "text-white/60" : "text-[#868E96]"}`}>{label}</p>
        {icon && <span className={featured ? "text-white/70" : valueClass}>{icon}</span>}
      </div>
      <p
        className={`mt-3 truncate font-mono text-lg font-semibold tracking-tight sm:text-xl ${featured ? "text-white" : valueClass}`}
      >
        {value}
      </p>
      <p className={`mt-2 truncate text-[11px] ${featured ? "text-white/45" : "text-[#ADB5BD]"}`}>
        {hint}
      </p>
    </div>
  )
}
