import React, { useState } from "react"
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts"
import { AssetId, ASSET_DEFINITIONS, AvailableFund, ColorScheme, Portfolio } from "../types"
import { formatCurrency, formatCurrencyByCode, formatPercent, getProfitColor } from "../utils"
import FundOperationDialog from "./FundOperationDialog"

type OperationType = "transfer_in" | "transfer_out" | "transfer" | "convert"

interface DashboardProps {
  assets: Record<AssetId, number>
  total: number
  totalAssets: number
  principal: number
  totalFees: number
  colorScheme: ColorScheme
  availableFunds: AvailableFund[]
  exchangeRates: Record<string, number>
  portfolios: Portfolio[]
  currentPortfolioId: string
  onRefreshFunds: () => void
  displayCurrency: string
}

export default function Dashboard({ assets, total, totalAssets, principal, totalFees, colorScheme, availableFunds, exchangeRates, portfolios, currentPortfolioId, onRefreshFunds, displayCurrency }: DashboardProps) {
  const [showDetails, setShowDetails] = useState(false)
  const [fundOperation, setFundOperation] = useState<{ type: OperationType; currency?: string } | null>(null)

  const chartData = Object.keys(assets)
    .map((key) => {
      const id = key as AssetId
      const value = assets[id]
      return {
        name: ASSET_DEFINITIONS[id].name,
        value,
        color: ASSET_DEFINITIONS[id].color,
      }
    })
    .filter((item) => item.value > 0)

  const profit = totalAssets - principal
  const returnRate = principal > 0 ? profit / principal : 0
  const isPositive = profit >= 0

  const totalCNY = availableFunds.reduce((sum, f) => {
    const rate = exchangeRates[f.currency]
    return rate ? sum + f.amount * rate : sum
  }, 0)

  return (
    <div className="bg-white p-6 sm:p-8 rounded-2xl border border-[#E9ECEF] shadow-sm flex flex-col h-full">
      <div className="flex items-center justify-between mb-2">
        <p className="text-xs uppercase tracking-[0.2em] text-[#6C757D] font-semibold">
          总资产净值
        </p>

        <div className="flex items-center gap-2 text-xs text-[#ADB5BD]">
          <span>
            累计投入成本: <span className="font-mono">{formatCurrencyByCode(principal, displayCurrency)}</span>
          </span>
          {totalFees > 0 && (
            <span className="ml-2">
              累计手续费: <span className="font-mono">{formatCurrencyByCode(totalFees, displayCurrency)}</span>
            </span>
          )}
        </div>
      </div>

      <h2 className="text-4xl font-light mb-2 text-[#1A1A1A]">{formatCurrencyByCode(total, displayCurrency)}</h2>

      <div
        className={`flex items-center gap-2 mb-8 ${getProfitColor(isPositive, colorScheme)} font-mono`}
      >
        {isPositive ? (
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2"
              d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6"
            ></path>
          </svg>
        ) : (
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2"
              d="M13 17h8m0 0v-8m0 8l-8-8-4 4-6-6"
            ></path>
          </svg>
        )}
        <span className="text-sm font-medium">
          {isPositive ? "+" : ""}
          {formatPercent(returnRate)} ({isPositive ? "+" : ""}
          {formatCurrencyByCode(profit, displayCurrency)})
        </span>
      </div>

      <div className="w-full h-64 relative flex justify-center">
        {total > 0 ? (
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={chartData}
                cx="50%"
                cy="50%"
                innerRadius={70}
                outerRadius={95}
                paddingAngle={2}
                dataKey="value"
                stroke="none"
              >
                {chartData.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={entry.color} />
                ))}
              </Pie>
              <Tooltip
                formatter={(value) => formatCurrencyByCode(Number(value), displayCurrency)}
                contentStyle={{
                  borderRadius: "0.5rem",
                  border: "1px solid #E9ECEF",
                  boxShadow: "0 4px 6px -1px rgb(0 0 0 / 0.05)",
                  backgroundColor: "#fff",
                }}
              />
            </PieChart>
          </ResponsiveContainer>
        ) : (
          <div className="absolute inset-0 flex flex-col items-center justify-center">
            <p className="text-2xl font-light text-[#1A1A1A]">0.00</p>
            <p className="text-[10px] text-[#ADB5BD] uppercase tracking-wider mt-1">暂无数据</p>
          </div>
        )}
        {total > 0 && (
          <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
            <p className={`text-2xl font-light ${getProfitColor(isPositive, colorScheme)}`}>
              {isPositive ? "+" : ""}{formatPercent(returnRate)}
            </p>
            <p className="text-[10px] text-[#ADB5BD] uppercase tracking-wider mt-1">累计收益</p>
          </div>
        )}
      </div>

      <div className="mt-8 grid grid-cols-2 gap-4">
        {Object.keys(ASSET_DEFINITIONS).map((key) => {
          const id = key as AssetId
          const value = assets[id] || 0
          const percentage = totalAssets > 0 ? value / totalAssets : 0
          const def = ASSET_DEFINITIONS[id]

          return (
            <div key={id} className="flex items-center gap-3">
              <div
                className={`w-2 h-2 rounded-full ${id === "cash" ? "border border-[#ADB5BD]" : ""}`}
                style={{ backgroundColor: def.color }}
              />
              <span className="text-xs text-[#495057] font-medium">
                {def.name}{" "}
                <span className="font-normal text-[#ADB5BD]">({formatPercent(percentage)})</span>
              </span>
            </div>
          )
        })}
      </div>

      <div className="mt-6 pt-4 border-t border-[#E9ECEF]">
        <div className="flex items-center justify-between">
          <span className="text-xs text-[#6C757D]">可用资金</span>
          <button
            onClick={() => setShowDetails(!showDetails)}
            className="text-sm font-mono text-[#1A1A1A] hover:text-blue-600 transition-colors cursor-pointer"
            title="点击查看详情"
          >
            {formatCurrency(totalCNY)}
            <span className="text-[10px] ml-1 text-[#ADB5BD]">{showDetails ? "▲" : "▼"}</span>
          </button>
        </div>

        {showDetails && (
          <div className="mt-3 space-y-2">
            {availableFunds.map((f) => (
              <div key={f.currency} className="flex items-center justify-between pl-2">
                <span className="text-xs text-[#6C757D]">{f.currency}</span>
                <div className="flex items-center gap-2">
                  <span className="text-xs font-mono text-[#1A1A1A]">
                    {formatCurrencyByCode(f.amount, f.currency)}
                  </span>
                  <div className="flex gap-1">
                    <button
                      onClick={() => setFundOperation({ type: "transfer_in", currency: f.currency })}
                      className="text-[10px] text-green-600 hover:text-green-800 transition-colors"
                      title="转入"
                    >
                      转入
                    </button>
                    <button
                      onClick={() => setFundOperation({ type: "transfer_out", currency: f.currency })}
                      className="text-[10px] text-red-600 hover:text-red-800 transition-colors"
                      title="转出"
                    >
                      转出
                    </button>
                  </div>
                </div>
              </div>
            ))}

            {availableFunds.length === 0 && (
              <div className="flex items-center justify-between pl-2">
                <span className="text-xs text-[#6C757D]">暂无可用资金</span>
                <button
                  onClick={() => setFundOperation({ type: "transfer_in", currency: "CNY" })}
                  className="text-[10px] text-green-600 hover:text-green-800 transition-colors"
                >
                  转入
                </button>
              </div>
            )}

            <div className="flex gap-2 pl-2 pt-2 border-t border-[#F1F3F5]">
              <button
                onClick={() => setFundOperation({ type: "transfer" })}
                className="text-[10px] px-2 py-1 border border-[#E9ECEF] rounded text-[#495057] hover:bg-[#F8F9FA] transition-colors"
              >
                划转
              </button>
              <button
                onClick={() => setFundOperation({ type: "convert" })}
                className="text-[10px] px-2 py-1 border border-[#E9ECEF] rounded text-[#495057] hover:bg-[#F8F9FA] transition-colors"
              >
                货币转换
              </button>
            </div>
          </div>
        )}
      </div>

      {fundOperation && (
        <FundOperationDialog
          type={fundOperation.type}
          portfolios={portfolios}
          currentPortfolioId={currentPortfolioId}
          currentCurrency={fundOperation.currency}
          onClose={() => setFundOperation(null)}
          onSuccess={onRefreshFunds}
        />
      )}
    </div>
  )
}
