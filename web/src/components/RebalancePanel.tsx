import React from "react"
import { AssetId, ASSET_DEFINITIONS, ColorScheme } from "../types"
import { formatCurrency, getProfitColor } from "../utils"

interface RebalancePanelProps {
  assets: Record<AssetId, number>
  total: number
  driftThreshold: number
  colorScheme: ColorScheme
  targetPcts: Record<AssetId, number>
}

export default function RebalancePanel({ assets, total, driftThreshold, colorScheme, targetPcts }: RebalancePanelProps) {
  if (total === 0) {
    return null
  }

  const items = Object.keys(ASSET_DEFINITIONS).map((key) => {
    const id = key as AssetId
    const def = ASSET_DEFINITIONS[id]
    const targetPct = targetPcts[id]
    const currentValue = assets[id] || 0
    const currentPct = total > 0 ? currentValue / total : 0
    const targetValue = total * (targetPct / 100)
    const difference = targetValue - currentValue
    const driftPct = (currentPct * 100) - targetPct
    const isBalanced = Math.abs(driftPct) < driftThreshold

    return {
      id,
      def,
      targetPct,
      currentPct,
      currentValue,
      targetValue,
      difference,
      driftPct,
      isBalanced,
      action: difference > 0 ? "buy" : difference < 0 ? "sell" : "keep",
    }
  })

  const needsAction = items.filter((i) => !i.isBalanced)
  const allBalanced = needsAction.length === 0

  return (
    <div className="bg-white rounded-2xl border border-[#E9ECEF] shadow-sm flex flex-col overflow-hidden h-full">
      <div className="p-6 border-b border-[#F1F3F5] flex justify-between items-center bg-white">
        <h3 className="text-lg font-medium text-[#1A1A1A]">再平衡建议</h3>
        <span className="text-[10px] uppercase tracking-widest text-[#ADB5BD] border border-[#E9ECEF] px-2 py-1 rounded-sm bg-[#F8F9FA]">
          ±{driftThreshold}% 漂移
        </span>
      </div>

      <div className="p-6 bg-white flex flex-col gap-3 overflow-auto">
        {allBalanced && (
          <div className="flex items-center gap-3 p-3 rounded-lg bg-[#F8F9FA] border border-[#E9ECEF] mb-1">
            <svg className="w-4 h-4 text-emerald-500 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <p className="text-xs text-[#6C757D]">资产比例健康，无需调整</p>
          </div>
        )}

        {items.map((item) => {
          const currentWidth = Math.min(item.currentPct * 100, 100)
          const targetWidth = Math.min(item.targetPct, 100)
          const isOver = item.driftPct > 0

          return (
            <div
              key={item.id}
              className={`p-4 rounded-xl border transition-colors ${
                item.isBalanced
                  ? "border-[#E9ECEF] hover:bg-[#F8F9FA]"
                  : item.action === "buy"
                    ? "border-[#1A1A1A]/10 bg-[#F8F9FA]"
                    : "border-orange-200 bg-orange-50/30"
              }`}
            >
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-3">
                  <div
                    className={`w-8 h-8 rounded flex items-center justify-center text-[9px] font-bold ${item.id === "cash" ? "text-[#495057] border border-[#DEE2E6]" : "text-white"}`}
                    style={{ backgroundColor: item.def.color }}
                  >
                    {item.id === "stocks" ? "STK" : item.id === "bonds" ? "BND" : item.id === "gold" ? "CMD" : "CSH"}
                  </div>
                  <div>
                    <p className="text-sm font-medium text-[#1A1A1A]">{item.def.name}</p>
                    <p className="text-[11px] text-[#ADB5BD] font-mono">{formatCurrency(item.currentValue)}</p>
                  </div>
                </div>

                <div className="flex items-center gap-3">
                  {!item.isBalanced && (
                    <span className={`text-[11px] font-mono font-medium ${isOver ? getProfitColor(false, colorScheme) : "text-[#1A1A1A]"}`}>
                      {isOver ? "+" : ""}{item.driftPct.toFixed(1)}%
                    </span>
                  )}
                  <span
                    className={`inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-[11px] font-medium tracking-wide
                      ${item.action === "buy"
                        ? "bg-[#1A1A1A] text-white"
                        : item.action === "sell"
                          ? `bg-white border border-orange-200 ${getProfitColor(false, colorScheme)}`
                          : "bg-[#F8F9FA] text-[#ADB5BD] border border-[#E9ECEF]"
                      }`}
                  >
                    {item.action === "buy" ? "补仓" : item.action === "sell" ? "减仓" : "保持"}
                    {item.action !== "keep" && (
                      <span className="font-mono">{formatCurrency(Math.abs(item.difference))}</span>
                    )}
                  </span>
                </div>
              </div>

              <div className="relative h-1.5 bg-[#F1F3F5] rounded-full overflow-hidden">
                <div
                  className="absolute inset-y-0 left-0 rounded-full transition-all duration-500"
                  style={{
                    width: `${currentWidth}%`,
                    backgroundColor: item.def.color === "#E9ECEF" ? "#ADB5BD" : item.def.color,
                    opacity: 0.7,
                  }}
                />
                <div
                  className="absolute inset-y-0 w-0.5 bg-[#1A1A1A]/40 transition-all duration-500"
                  style={{ left: `${targetWidth}%` }}
                />
              </div>

              <div className="flex justify-between mt-1.5">
                <span className="text-[10px] text-[#ADB5BD] font-mono">
                  当前 {(item.currentPct * 100).toFixed(1)}%
                </span>
                <span className="text-[10px] text-[#6C757D] font-mono">
                  目标 {item.targetPct}%
                </span>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
