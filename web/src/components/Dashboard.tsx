import React, { useState, useEffect, useCallback } from "react"
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts"
import { AssetId, ASSET_DEFINITIONS, AvailableFund, ColorScheme } from "../types"
import { formatCurrency, formatCurrencyByCode, formatPercent, getProfitColor } from "../utils"
import * as api from "../api"

interface DashboardProps {
  assets: Record<AssetId, number>
  total: number
  totalAssets: number
  principal: number
  totalFees: number
  colorScheme: ColorScheme
  availableFunds: AvailableFund[]
  onUpdateAvailableFunds: (currency: string, amount: number) => void
}

const SUPPORTED_CURRENCIES = ["CNY", "USD", "HKD", "EUR", "GBP", "JPY"]

export default function Dashboard({ assets, total, totalAssets, principal, totalFees, colorScheme, availableFunds, onUpdateAvailableFunds }: DashboardProps) {
  const [isEditingFunds, setIsEditingFunds] = useState(false)
  const [editingCurrency, setEditingCurrency] = useState("")
  const [tempAmount, setTempAmount] = useState("")
  const [showDetails, setShowDetails] = useState(false)
  const [exchangeRates, setExchangeRates] = useState<Record<string, number>>({ CNY: 1 })

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

  useEffect(() => {
    const currencies = availableFunds.filter((f) => f.currency !== "CNY").map((f) => f.currency)
    const unique = [...new Set(currencies)]
    if (unique.length === 0) return
    Promise.all(
      unique.map(async (cur) => {
        try {
          const res = await api.fetchExchangeRate(`${cur}CNY`)
          return { currency: cur, rate: res.rate }
        } catch {
          return { currency: cur, rate: 1 }
        }
      })
    ).then((results) => {
      const rates: Record<string, number> = { CNY: 1 }
      results.forEach((r) => { rates[r.currency] = r.rate })
      setExchangeRates(rates)
    })
  }, [availableFunds])

  const totalCNY = availableFunds.reduce((sum, f) => {
    return sum + f.amount * (exchangeRates[f.currency] || 1)
  }, 0)

  const startEdit = useCallback((currency: string, amount: number) => {
    setEditingCurrency(currency)
    setTempAmount(amount.toString())
    setIsEditingFunds(true)
  }, [])

  const saveEdit = useCallback(() => {
    const num = parseFloat(tempAmount)
    if (!isNaN(num) && num >= 0) {
      onUpdateAvailableFunds(editingCurrency, num)
      setIsEditingFunds(false)
    }
  }, [editingCurrency, tempAmount, onUpdateAvailableFunds])

  const usedCurrencies = availableFunds.map((f) => f.currency)
  const availableNewCurrencies = SUPPORTED_CURRENCIES.filter((c) => !usedCurrencies.includes(c))

  return (
    <div className="bg-white p-6 sm:p-8 rounded-2xl border border-[#E9ECEF] shadow-sm flex flex-col h-full">
      <div className="flex items-center justify-between mb-2">
        <p className="text-xs uppercase tracking-[0.2em] text-[#6C757D] font-semibold">
          总资产净值
        </p>

        <div className="flex items-center gap-2 text-xs text-[#ADB5BD]">
          <span>
            累计投入成本: <span className="font-mono">{formatCurrency(principal)}</span>
          </span>
          {totalFees > 0 && (
            <span className="ml-2">
              累计手续费: <span className="font-mono">{formatCurrency(totalFees)}</span>
            </span>
          )}
        </div>
      </div>

      <h2 className="text-4xl font-light mb-2 text-[#1A1A1A]">{formatCurrency(total)}</h2>

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
          {formatCurrency(profit)})
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
                formatter={(value) => formatCurrency(Number(value))}
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
                {isEditingFunds && editingCurrency === f.currency ? (
                  <div className="flex items-center gap-2 w-full">
                    <span className="text-xs text-[#6C757D] w-10">{f.currency}</span>
                    <input
                      type="number"
                      value={tempAmount}
                      onChange={(e) => setTempAmount(e.target.value)}
                      className="flex-1 px-2 py-1 text-xs text-right border border-[#E9ECEF] rounded focus:outline-none focus:border-[#1A1A1A] font-mono"
                      autoFocus
                      onKeyDown={(e) => {
                        if (e.key === "Enter") saveEdit()
                        else if (e.key === "Escape") setIsEditingFunds(false)
                      }}
                    />
                    <button onClick={saveEdit} className="text-[10px] text-white bg-[#1A1A1A] px-2 py-1 rounded hover:opacity-90">
                      保存
                    </button>
                    <button onClick={() => setIsEditingFunds(false)} className="text-[10px] text-[#ADB5BD] hover:text-[#1A1A1A]">
                      取消
                    </button>
                  </div>
                ) : (
                  <>
                    <span className="text-xs text-[#6C757D]">{f.currency}</span>
                    <button
                      onClick={() => startEdit(f.currency, f.amount)}
                      className="text-xs font-mono text-[#1A1A1A] hover:text-blue-600 transition-colors cursor-pointer"
                      title="点击编辑"
                    >
                      {formatCurrencyByCode(f.amount, f.currency)}
                    </button>
                  </>
                )}
              </div>
            ))}

            {availableFunds.length === 0 && (
              <div className="flex items-center justify-between pl-2">
                {isEditingFunds && editingCurrency === "CNY" ? (
                  <div className="flex items-center gap-2 w-full">
                    <span className="text-xs text-[#6C757D] w-10">CNY</span>
                    <input
                      type="number"
                      value={tempAmount}
                      onChange={(e) => setTempAmount(e.target.value)}
                      className="flex-1 px-2 py-1 text-xs text-right border border-[#E9ECEF] rounded focus:outline-none focus:border-[#1A1A1A] font-mono"
                      autoFocus
                      onKeyDown={(e) => {
                        if (e.key === "Enter") saveEdit()
                        else if (e.key === "Escape") setIsEditingFunds(false)
                      }}
                    />
                    <button onClick={saveEdit} className="text-[10px] text-white bg-[#1A1A1A] px-2 py-1 rounded hover:opacity-90">
                      保存
                    </button>
                    <button onClick={() => setIsEditingFunds(false)} className="text-[10px] text-[#ADB5BD] hover:text-[#1A1A1A]">
                      取消
                    </button>
                  </div>
                ) : (
                  <>
                    <span className="text-xs text-[#6C757D]">CNY</span>
                    <button
                      onClick={() => startEdit("CNY", 0)}
                      className="text-xs font-mono text-[#ADB5BD] hover:text-blue-600 transition-colors cursor-pointer"
                      title="点击设置"
                    >
                      ¥0.00
                    </button>
                  </>
                )}
              </div>
            )}

            {availableNewCurrencies.length > 0 && (
              <div className="pl-2 pt-1">
                <select
                  className="text-[10px] text-[#ADB5BD] border border-[#E9ECEF] rounded px-1 py-0.5 focus:outline-none"
                  value=""
                  onChange={(e) => {
                    if (e.target.value) {
                      startEdit(e.target.value, 0)
                    }
                  }}
                >
                  <option value="">+ 添加币种</option>
                  {availableNewCurrencies.map((c) => (
                    <option key={c} value={c}>{c}</option>
                  ))}
                </select>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
