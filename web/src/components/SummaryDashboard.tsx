import { PortfolioSummary, ASSET_DEFINITIONS, AssetId, ColorScheme } from "../types"
import { formatCurrencyByCode } from "../utils"

interface Props {
  summary: PortfolioSummary | null
  colorScheme: ColorScheme
  displayCurrency: string
  onClose: () => void
}

export default function SummaryDashboard({ summary, colorScheme, displayCurrency, onClose }: Props) {
  if (!summary) return null

  const greenUp = colorScheme === "green-up"
  const pnlColor = (val: number) => {
    if (val > 0) return greenUp ? "text-green-600" : "text-red-600"
    if (val < 0) return greenUp ? "text-red-600" : "text-green-600"
    return "text-[#6C757D]"
  }

  const pnl =
    summary.principal > 0 ? ((summary.total - summary.principal) / summary.principal) * 100 : 0

  return (
    <div
      className="fixed inset-0 bg-black/30 flex items-center justify-center z-50"
      onClick={onClose}
    >
      <div
        className="bg-white rounded-lg shadow-lg p-6 w-full max-w-lg max-h-[80vh] overflow-y-auto"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">投资组合汇总</h2>
          <button
            onClick={onClose}
            className="text-[#6C757D] hover:text-[#1A1A1A] text-xl leading-none"
          >
            &times;
          </button>
        </div>

        <div className="grid grid-cols-3 gap-4 mb-6">
          <div>
            <p className="text-xs text-[#6C757D]">总资产</p>
            <p className="text-lg font-semibold">
              {formatCurrencyByCode(summary.total, displayCurrency)}
            </p>
          </div>
          <div>
            <p className="text-xs text-[#6C757D]">总投入</p>
            <p className="text-lg font-semibold">
              {formatCurrencyByCode(summary.principal, displayCurrency)}
            </p>
          </div>
          <div>
            <p className="text-xs text-[#6C757D]">总收益</p>
            <p className={`text-lg font-semibold ${pnlColor(pnl)}`}>
              {pnl > 0 ? "+" : ""}
              {pnl.toFixed(1)}%
            </p>
          </div>
        </div>

        <h3 className="text-sm font-medium mb-2">资产配置</h3>
        <div className="space-y-1.5 mb-6">
          {(["stocks", "bonds", "cash", "commodities"] as AssetId[]).map((id) => {
            const val = summary.assets[id] || 0
            const pct = summary.total > 0 ? (val / summary.total) * 100 : 0
            return (
              <div key={id} className="flex items-center gap-2 text-sm">
                <div
                  className="w-2 h-2 rounded-full"
                  style={{ backgroundColor: ASSET_DEFINITIONS[id].color }}
                />
                <span className="w-16 text-[#6C757D]">{ASSET_DEFINITIONS[id].name}</span>
                <div className="flex-1 h-2 bg-[#F8F9FA] rounded-full overflow-hidden">
                  <div
                    className="h-full rounded-full"
                    style={{ width: `${pct}%`, backgroundColor: ASSET_DEFINITIONS[id].color }}
                  />
                </div>
                <span className="w-12 text-right text-xs">{pct.toFixed(1)}%</span>
                <span className="w-20 text-right text-xs text-[#6C757D]">
                  {formatCurrencyByCode(val, displayCurrency)}
                </span>
              </div>
            )
          })}
        </div>

        <h3 className="text-sm font-medium mb-2">各组合概况</h3>
        <div className="space-y-2">
          {summary.portfolios.map((p) => {
            const pPnl = p.principal > 0 ? ((p.total - p.principal) / p.principal) * 100 : 0
            return (
              <div key={p.id} className="border border-[#E9ECEF] rounded p-3">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">{p.name}</span>
                  <span className="text-sm">
                    {formatCurrencyByCode(p.total, displayCurrency)}
                  </span>
                </div>
                <div className="flex items-center justify-between mt-1">
                  <span className="text-xs text-[#6C757D]">
                    投入 {formatCurrencyByCode(p.principal, displayCurrency)}
                  </span>
                  <span className={`text-xs ${pnlColor(pPnl)}`}>
                    {pPnl > 0 ? "+" : ""}
                    {pPnl.toFixed(1)}%
                  </span>
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
