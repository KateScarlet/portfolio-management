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
    const targetValue = total * (targetPcts[id] / 100)
    const currentValue = assets[id] || 0
    const difference = targetValue - currentValue
    const isBalanced = Math.abs(difference / total) < driftThreshold / 100 // Within drift tolerance

    return {
      id,
      def,
      targetValue,
      currentValue,
      difference,
      isBalanced,
      action: difference > 0 ? "buy" : difference < 0 ? "sell" : "keep",
    }
  })

  const allBalanced = items.every((i) => i.isBalanced)

  return (
    <div className="bg-white rounded-2xl border border-[#E9ECEF] shadow-sm flex flex-col overflow-hidden h-full">
      <div className="p-6 border-b border-[#F1F3F5] flex justify-between items-center bg-white">
        <h3 className="text-lg font-medium text-[#1A1A1A]">再平衡建议</h3>
        <span className="text-[10px] uppercase tracking-widest text-[#ADB5BD] border border-[#E9ECEF] px-2 py-1 rounded-sm bg-[#F8F9FA]">
          ±{driftThreshold}% 漂移
        </span>
      </div>

      <div className="p-6 bg-white space-y-4">
        {allBalanced && (
          <div className="p-4 bg-white border border-[#E9ECEF] rounded-lg flex items-center gap-4 mb-2">
            <div className="w-10 h-10 rounded-full bg-[#F8F9FA] flex items-center justify-center shrink-0">
              <svg
                className="w-5 h-5 text-[#1A1A1A]"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2"
                  d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                ></path>
              </svg>
            </div>
            <div>
              <p className="text-sm font-semibold text-[#1A1A1A]">资产比例健康</p>
              <p className="text-xs text-[#6C757D]">当前配置处于完美平衡状态，无需进行任何操作。</p>
            </div>
          </div>
        )}

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {items.map((item) => (
            <div
              key={item.id}
              className="p-4 rounded-xl border border-[#E9ECEF] hover:bg-[#F8F9FA] transition-colors flex flex-col justify-between gap-4"
            >
              <div className="flex items-center gap-4">
                <div
                  className={`w-10 h-10 rounded flex items-center justify-center text-[10px] font-bold ${item.id === "cash" ? "text-[#495057] border border-[#DEE2E6]" : "text-white"}`}
                  style={{ backgroundColor: item.def.color }}
                >
                  {item.id === "stocks"
                    ? "STK"
                    : item.id === "bonds"
                      ? "BND"
                      : item.id === "gold"
                        ? "CMD"
                        : "CSH"}
                </div>
                <div>
                  <p className="text-sm font-medium text-[#1A1A1A]">{item.def.name}</p>
                  <div className="text-xs text-[#ADB5BD] mt-0.5 flex flex-wrap items-center gap-2">
                    <span className="font-mono">{formatCurrency(item.currentValue)}</span>
                    <svg
                      className="w-3 h-3 text-[#ADB5BD]"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth="2"
                        d="M14 5l7 7m0 0l-7 7m7-7H3"
                      ></path>
                    </svg>
                    <span className="font-mono font-medium text-[#6C757D]">
                      {formatCurrency(item.targetValue)}
                    </span>
                  </div>
                </div>
              </div>

              <div
                className={`flex items-center justify-center gap-2 px-4 py-2 rounded-full font-medium text-[11px] w-full border tracking-wide uppercase
                ${
                  item.action === "buy"
                    ? "bg-[#1A1A1A] text-white border-[#1A1A1A]"
                    : item.action === "sell"
                      ? `bg-white ${getProfitColor(false, colorScheme)} border-orange-200`
                      : "bg-[#F8F9FA] text-[#6C757D] border-[#E9ECEF]"
                }`}
              >
                <span>
                  {item.action === "buy"
                    ? "补仓 · "
                    : item.action === "sell"
                      ? "减仓 · "
                      : "保持 · "}
                  {item.action !== "keep" ? formatCurrency(Math.abs(item.difference)) : "平衡"}
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
