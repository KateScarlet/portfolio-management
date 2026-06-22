import React, { useState, useCallback, useRef } from "react"
import { ASSET_DEFINITIONS, Holding, HoldingLot, ColorScheme } from "../types"
import { formatCurrency, formatPercent, getProfitColor } from "../utils"
import * as api from "../api"
import AddHoldingForm from "./AddHoldingForm"
import SellModal from "./SellModal"
import ConfirmDialog from "./ConfirmDialog"
import { useToast } from "./toast-context"

interface HoldingsManagerProps {
  holdings: Holding[]
  setHoldings: React.Dispatch<React.SetStateAction<Holding[]>>
  total: number
  onAddHolding: (holding: Omit<Holding, "id">) => Promise<void>
  onUpdateHolding: (id: string, updates: Partial<Holding>) => void
  onRemoveHolding: (id: string) => void
  onSaveRecord: () => void
  colorScheme: ColorScheme
  onRefreshAvailableFunds: () => Promise<void>
}

export default function HoldingsManager({
  holdings,
  setHoldings,
  total: _total,
  onAddHolding,
  onUpdateHolding,
  onRemoveHolding,
  onSaveRecord,
  colorScheme,
  onRefreshAvailableFunds,
}: HoldingsManagerProps) {
  const [isAdding, setIsAdding] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [editingValueId, setEditingValueId] = useState<string | null>(null)
  const [tempEditValue, setTempEditValue] = useState("")
  const [editingLotId, setEditingLotId] = useState<string | null>(null)
  const [editingLotCost, setEditingLotCost] = useState("")
  const [editingLotValue, setEditingLotValue] = useState("")
  const [editingLotFee, setEditingLotFee] = useState("")
  const [editingLotShares, setEditingLotShares] = useState("")
  const [editingLotCostPrice, setEditingLotCostPrice] = useState("")
  const [editingLotCurrency, setEditingLotCurrency] = useState("CNY")
  const [sellingHolding, setSellingHolding] = useState<Holding | null>(null)
  const [deletingHolding, setDeletingHolding] = useState<Holding | null>(null)

  const { showToast } = useToast()

  const fxRateRef = useRef<Record<string, number>>({})

  const computeCost = useCallback(
    (costPriceStr: string, sharesStr: string, currency: string) => {
      const p = parseFloat(costPriceStr) || 0
      const s = parseFloat(sharesStr) || 0
      if (currency === "CNY") {
        return String(p * s)
      }
      const rate = fxRateRef.current[currency] || 0
      return rate > 0 ? String(p * rate * s) : String(p * s)
    },
    []
  )

  const syncAllPrices = useCallback(async () => {
    setSyncing(true)
    try {
      await api.triggerSync()
    } finally {
      setSyncing(false)
    }
  }, [])

  const saveEditLot = useCallback(
    (h: Holding, lotId: string, updatedFields: Partial<HoldingLot>) => {
      if (!h.lots) return
      const updatedLots = h.lots.map((l) =>
        l.id === lotId ? { ...l, ...updatedFields } : l
      )
      // Send only lots to backend; backend recalculates all derived fields
      onUpdateHolding(h.id, { lots: updatedLots })
      setEditingLotId(null)
    },
    [onUpdateHolding]
  )

  const deleteEditLot = useCallback(
    (h: Holding, lotId: string) => {
      if (!h.lots) return
      const updatedLots = h.lots.filter((l) => l.id !== lotId)
      if (updatedLots.length === 0) {
        onRemoveHolding(h.id)
      } else {
        onUpdateHolding(h.id, { lots: updatedLots })
      }
      setEditingLotId(null)
    },
    [onUpdateHolding, onRemoveHolding]
  )

  const handleSellConfirm = useCallback(
    (soldHolding: Holding) => {
      setHoldings((prev) =>
        prev.map((h) => {
          if (h.id === soldHolding.id) return soldHolding
          return h
        })
      )
      onRefreshAvailableFunds()
    },
    [setHoldings, onRefreshAvailableFunds]
  )

  return (
    <div className="bg-white rounded-2xl border border-[#E9ECEF] shadow-sm flex flex-col overflow-hidden">
      <div className="p-6 border-b border-[#F1F3F5] flex justify-between items-center bg-white flex-wrap gap-4">
        <h3 className="text-lg font-medium text-[#1A1A1A]">资产明细</h3>
        <div className="flex gap-2">
          <button
            onClick={syncAllPrices}
            disabled={syncing}
            className="text-xs bg-[#F8F9FA] border border-[#DEE2E6] text-[#1A1A1A] px-4 py-2 rounded-full hover:bg-gray-50 font-medium transition-colors disabled:opacity-50"
          >
            {syncing ? "同步中..." : "同步价格"}
          </button>
          <button
            onClick={onSaveRecord}
            className="text-xs bg-[#F8F9FA] border border-[#DEE2E6] text-[#1A1A1A] px-4 py-2 rounded-full hover:bg-gray-50 font-medium transition-colors"
          >
            保存记录
          </button>
          <button
            onClick={() => setIsAdding(!isAdding)}
            className="text-xs bg-[#1A1A1A] text-white px-4 py-2 rounded-full hover:opacity-90 transition-opacity"
          >
            {isAdding ? "取消" : "+ 录入资产"}
          </button>
        </div>
      </div>

      {isAdding && (
        <AddHoldingForm
          onAddHolding={onAddHolding}
          onClose={() => setIsAdding(false)}
        />
      )}

      <div className="grow overflow-x-auto">
        <table className="w-full text-left">
          <thead className="text-[10px] uppercase tracking-widest text-[#ADB5BD] border-b border-[#F1F3F5] bg-white">
            <tr>
              <th className="px-6 py-4 font-bold">资产大类</th>
              <th className="px-6 py-4 font-bold">代码/名称</th>
              <th className="px-6 py-4 font-bold text-right">单价 & 份额</th>
              <th className="px-6 py-4 font-bold text-right">总成本 & 盈亏</th>
              <th className="px-6 py-4 font-bold text-right">当前总市值</th>
              <th className="px-6 py-4 font-bold text-right">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[#F8F9FA] bg-white text-[#1A1A1A]">
            {holdings.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-6 py-12 text-center text-sm text-[#ADB5BD]">
                  暂无资产明细，请在上方点击"录入资产"开始构建组合。
                </td>
              </tr>
            ) : (
              holdings.map((h) => {
                const def = ASSET_DEFINITIONS[h.assetId]
                const isExpanded = expandedId === h.id

                return (
                  <React.Fragment key={h.id}>
                    <tr
                      className="hover:bg-[#F8F9FA] transition-colors group cursor-pointer"
                      onClick={() => setExpandedId(isExpanded ? null : h.id)}
                    >
                      <td className="px-6 py-5 flex items-center gap-3">
                        <div
                          className={`w-8 h-8 rounded flex items-center justify-center text-[10px] font-bold ${h.assetId === "cash" ? "text-[#495057] border border-[#DEE2E6]" : "text-white"}`}
                          style={{ backgroundColor: def.color }}
                        >
                          {h.assetId === "stocks"
                            ? "STK"
                            : h.assetId === "bonds"
                              ? "BND"
                              : h.assetId === "gold"
                                ? "CMD"
                                : "CSH"}
                        </div>
                        <div>
                          <p className="text-sm font-medium">{def.name}</p>
                        </div>
                      </td>
                      <td className="px-6 py-5">
                        {h.symbol ? (
                          <div>
                            <p className="text-sm font-mono flex items-center gap-2">
                              {h.symbol}
                              {h.lots && h.lots.length > 0 && (
                                <span className="text-[10px] bg-gray-100 text-gray-500 px-1.5 py-0.5 rounded">
                                  {h.lots.length} 笔
                                </span>
                              )}
                            </p>
                            <p
                              className="text-[10px] text-[#ADB5BD] truncate max-w-37.5"
                              title={h.name}
                            >
                              {h.name}
                            </p>
                          </div>
                        ) : (
                          <div className="flex items-center gap-2">
                            <p className="text-sm text-[#6C757D]">{h.name || "手工资产"}</p>
                            {h.lots && h.lots.length > 0 && (
                              <span className="text-[10px] bg-gray-100 text-gray-500 px-1.5 py-0.5 rounded">
                                {h.lots.length} 笔
                              </span>
                            )}
                          </div>
                        )}
                      </td>
                      <td className="px-6 py-5 text-right font-mono text-sm text-[#495057]">
                        {h.symbol ? (
                          <div>
                            <p>{formatCurrency(h.price)}</p>
                            <p className="text-[10px] text-[#ADB5BD]">× {h.shares}</p>
                          </div>
                        ) : (h.shares || 0) > 0 ? (
                          <div>
                            {(h.costPrice || 0) > 0 && <p>{formatCurrency(h.costPrice || 0)}</p>}
                            <p className="text-[10px] text-[#ADB5BD]">× {h.shares}</p>
                          </div>
                        ) : (
                          <span className="text-[#ADB5BD] text-xs">-</span>
                        )}
                      </td>
                      <td className="px-6 py-5 text-right font-mono text-sm text-[#495057]">
                        {h.cost && h.cost > 0 ? (
                          <div>
                            <p>{formatCurrency(h.cost)}</p>
                            {(() => {
                              const profit = h.value - h.cost
                              const returnRate = profit / h.cost
                              const isPositive = profit >= 0
                              return (
                                <p
                                  className={`text-[10px] ${getProfitColor(isPositive, colorScheme)}`}
                                >
                                  {isPositive ? "+" : ""}
                                  {formatPercent(returnRate)}
                                </p>
                              )
                            })()}
                          </div>
                        ) : (
                          <span className="text-[#ADB5BD] text-xs">-</span>
                        )}
                      </td>
                      <td
                        className="px-6 py-5 text-right font-medium text-sm font-mono"
                        onClick={(e) => e.stopPropagation()}
                      >
                        {editingValueId === h.id ? (
                          <div className="flex items-center justify-end gap-2">
                            <input
                              type="number"
                              value={tempEditValue}
                              onChange={(e) => setTempEditValue(e.target.value)}
                              className="w-24 px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A] font-mono text-right"
                              autoFocus
                            />
                            <button
                              onClick={() => {
                                const num = parseFloat(tempEditValue)
                                if (!isNaN(num) && num >= 0) {
                                  onUpdateHolding(h.id, { value: num })
                                  setEditingValueId(null)
                                }
                              }}
                              className="text-[10px] text-white bg-[#1A1A1A] px-2 py-1 rounded hover:opacity-90"
                            >
                              保存
                            </button>
                            <button
                              onClick={() => setEditingValueId(null)}
                              className="text-[10px] text-[#ADB5BD] hover:text-[#1A1A1A]"
                            >
                              取消
                            </button>
                          </div>
                        ) : (
                          formatCurrency(h.value)
                        )}
                      </td>
                      <td
                        className="px-6 py-5 text-right space-x-2"
                        onClick={(e) => e.stopPropagation()}
                      >
                        {!h.symbol && (
                          <button
                            onClick={() => {
                              setEditingValueId(h.id)
                              setTempEditValue(h.value.toString())
                            }}
                            className="text-[10px] uppercase tracking-wider text-[#1A1A1A] hover:text-blue-600 font-bold transition-colors"
                            title="更新估值"
                          >
                            Update
                          </button>
                        )}
                        {h.assetId !== "cash" && (
                          <button
                            onClick={() => setSellingHolding(h)}
                            className="text-[10px] uppercase tracking-wider text-[#1A1A1A] hover:text-orange-500 font-bold transition-colors"
                            title="卖出"
                          >
                            Sell
                          </button>
                        )}
                        <button
                          onClick={() => setDeletingHolding(h)}
                          className="text-[10px] uppercase tracking-wider text-[#ADB5BD] hover:text-orange-500 font-bold transition-colors"
                        >
                          Del
                        </button>
                      </td>
                    </tr>

                    {isExpanded && h.lots && h.lots.length > 0 && (
                      <tr className="bg-[#F8F9FA]">
                        <td colSpan={6} className="px-6 py-4">
                          <div className="pl-12 border-l-2 border-[#DEE2E6] ml-4 space-y-2">
                            <h5 className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold mb-2">
                              记录
                            </h5>
                            {h.lots.map((lot) => {
                              const isEditing = editingLotId === lot.id
                              return (
                                <div
                                  key={lot.id}
                                  className={`flex justify-between items-center text-xs font-mono border-b border-[#E9ECEF] last:border-0 pb-2 last:pb-0 whitespace-nowrap ${isEditing ? "bg-white -mx-2 px-2 py-2 rounded-lg border border-[#DEE2E6] flex-wrap" : "text-[#495057]"} ${lot.type === "sell" ? "text-orange-600!" : ""}`}
                                >
                                  {isEditing ? (
                                    <div className="flex flex-wrap items-center gap-3 w-full">
                                      <div className="flex flex-col gap-0.5">
                                        <span className="text-[9px] text-[#ADB5BD] uppercase">日期</span>
                                        <input
                                          type="date"
                                          value={new Date(lot.date).toISOString().split("T")[0]}
                                          onChange={(e) => {
                                            const dateVal = new Date(e.target.value).getTime() || lot.date
                                            saveEditLot(h, lot.id, { date: dateVal })
                                          }}
                                          className="px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A]"
                                        />
                                      </div>
                                      {h.symbol && (
                                        <>
                                          <div className="flex flex-col gap-0.5">
                                            <span className="text-[9px] text-[#ADB5BD] uppercase">单价</span>
                                            <div className="flex w-full">
                                              <select
                                                value={editingLotCurrency}
                                                onChange={async (e) => {
                                                  const newCurrency = e.target.value
                                                  setEditingLotCurrency(newCurrency)
                                                  if (newCurrency !== "CNY" && !fxRateRef.current[newCurrency]) {
                                                    try {
                                                      const fxData = await api.fetchExchangeRate(`${newCurrency}CNY`)
                                                      if (fxData && fxData.rate) {
                                                        fxRateRef.current[newCurrency] = fxData.rate
                                                      }
                                                    } catch {
                                                      showToast("汇率获取失败", "info")
                                                    }
                                                  }
                                                  setEditingLotCost(computeCost(editingLotCostPrice, editingLotShares, newCurrency))
                                                }}
                                                className="px-1 py-1 border border-r-0 border-[#E9ECEF] rounded-l text-[10px] bg-gray-50 focus:outline-none focus:border-[#1A1A1A]"
                                              >
                                                <option value="CNY">CNY</option>
                                                <option value="USD">USD</option>
                                                <option value="HKD">HKD</option>
                                              </select>
                                              <input
                                                type="number"
                                                placeholder="0"
                                                value={editingLotCostPrice}
                                                onChange={(e) => {
                                                  setEditingLotCostPrice(e.target.value)
                                                  setEditingLotCost(computeCost(e.target.value, editingLotShares, editingLotCurrency))
                                                }}
                                                className="w-20 px-2 py-1 border border-[#E9ECEF] rounded-r text-xs focus:outline-none focus:border-[#1A1A1A] font-mono"
                                              />
                                            </div>
                                          </div>
                                          <div className="flex flex-col gap-0.5">
                                            <span className="text-[9px] text-[#ADB5BD] uppercase">数量</span>
                                            <input
                                              type="number"
                                              placeholder="0"
                                              value={editingLotShares}
                                              onChange={(e) => {
                                                setEditingLotShares(e.target.value)
                                                setEditingLotCost(computeCost(editingLotCostPrice, e.target.value, editingLotCurrency))
                                              }}
                                              className="w-20 px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A] font-mono"
                                            />
                                          </div>
                                        </>
                                      )}
                                      {h.symbol ? (
                                        <div className="flex flex-col gap-0.5">
                                          <span className="text-[9px] text-[#ADB5BD] uppercase">成本 (CNY)</span>
                                          <input
                                            type="number"
                                            placeholder="0"
                                            value={editingLotCost}
                                            readOnly
                                            className="w-24 px-2 py-1 border border-[#E9ECEF] rounded text-xs font-mono bg-gray-50 text-[#6C757D] cursor-not-allowed"
                                          />
                                        </div>
                                      ) : (
                                        <>
                                          <div className="flex flex-col gap-0.5">
                                            <span className="text-[9px] text-[#ADB5BD] uppercase">成本</span>
                                            <input
                                              type="number"
                                              placeholder="0"
                                              value={editingLotCost}
                                              onChange={(e) => setEditingLotCost(e.target.value)}
                                              className="w-24 px-2 py-1 border border-[#E9ECEF] rounded text-xs font-mono focus:outline-none focus:border-[#1A1A1A]"
                                            />
                                          </div>
                                          <div className="flex flex-col gap-0.5">
                                            <span className="text-[9px] text-[#ADB5BD] uppercase">当前价值</span>
                                            <input
                                              type="number"
                                              placeholder="0"
                                              value={editingLotValue}
                                              onChange={(e) => setEditingLotValue(e.target.value)}
                                              className="w-24 px-2 py-1 border border-[#E9ECEF] rounded text-xs font-mono focus:outline-none focus:border-[#1A1A1A]"
                                            />
                                          </div>
                                        </>
                                      )}
                                      <div className="flex flex-col gap-0.5">
                                        <span className="text-[9px] text-[#ADB5BD] uppercase">手续费</span>
                                        <input
                                          type="number"
                                          placeholder="0"
                                          value={editingLotFee}
                                          onChange={(e) => setEditingLotFee(e.target.value)}
                                          className="w-20 px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A] font-mono"
                                        />
                                      </div>
                                      <div className="flex gap-1 ml-auto">
                                        <button
                                          onClick={() => {
                                            if (h.symbol) {
                                              const costPrice = parseFloat(editingLotCostPrice) || 0
                                              const shares = parseFloat(editingLotShares) || lot.shares
                                              const cost = parseFloat(editingLotCost) || 0
                                              saveEditLot(h, lot.id, { costPrice, shares, cost, fee: parseFloat(editingLotFee) || 0 })
                                            } else {
                                              const cost = parseFloat(editingLotCost) || 0
                                              const valueAdded = parseFloat(editingLotValue) || 0
                                              saveEditLot(h, lot.id, { cost, valueAdded, fee: parseFloat(editingLotFee) || 0 })
                                            }
                                          }}
                                          className="text-[10px] text-white bg-[#1A1A1A] px-2 py-1 rounded hover:opacity-90"
                                        >
                                          保存
                                        </button>
                                        <button
                                          onClick={() => setEditingLotId(null)}
                                          className="text-[10px] text-[#ADB5BD] hover:text-[#1A1A1A] px-1"
                                        >
                                          取消
                                        </button>
                                      </div>
                                    </div>
                                  ) : (
                                    <>
                                      <span className="flex items-center gap-2">
                                        {lot.type === "sell" ? (
                                          <span className="text-[9px] bg-orange-100 text-orange-600 px-1.5 py-0.5 rounded font-bold">
                                            卖出
                                          </span>
                                        ) : (
                                          <span className="text-[9px] bg-blue-100 text-blue-600 px-1.5 py-0.5 rounded font-bold">
                                            买入
                                          </span>
                                        )}
                                        {new Date(lot.date).toLocaleDateString()}
                                      </span>
                                      <div className="flex items-center gap-4 text-right shrink-0">
                                        {h.symbol ? (
                                          <>
                                            <span className="w-28 text-right">
                                              单价: {formatCurrency(lot.costPrice || 0)}
                                            </span>
                                            <span className="w-20 text-right">
                                              数量: {lot.shares}
                                            </span>
                                            <span className="w-28 font-medium text-[#1A1A1A] text-right">
                                              {lot.type === "sell" ? "收入" : "成本"}:{" "}
                                              {formatCurrency(lot.cost || 0)}
                                            </span>
                                          </>
                                        ) : (
                                          <span className="w-28 font-medium text-[#1A1A1A] text-right">
                                            {lot.type === "sell" ? "收入" : "价值"}:{" "}
                                            {formatCurrency(lot.valueAdded || lot.cost || 0)}
                                          </span>
                                        )}
                                        {(lot.fee || 0) > 0 && (
                                          <span className="w-20 text-[10px] text-[#ADB5BD] text-right">
                                            费: {formatCurrency(lot.fee || 0)}
                                          </span>
                                        )}
                                        <div className="flex gap-2 shrink-0">
                                          <button
                                            onClick={() => {
                                              setEditingLotId(lot.id)
                                              setEditingLotCost(String(lot.cost ?? 0))
                                              setEditingLotValue(String(lot.valueAdded ?? lot.cost ?? 0))
                                              setEditingLotFee(String(lot.fee || 0))
                                              setEditingLotShares(String(lot.shares))
                                              setEditingLotCostPrice(String(lot.costPrice || 0))
                                              setEditingLotCurrency("CNY")
                                            }}
                                            className="text-[10px] uppercase tracking-wider text-[#1A1A1A] hover:text-blue-600 font-bold transition-colors whitespace-nowrap"
                                          >
                                            Edit
                                          </button>
                                          <button
                                            onClick={() => deleteEditLot(h, lot.id)}
                                            className="text-[10px] uppercase tracking-wider text-[#ADB5BD] hover:text-orange-500 font-bold transition-colors whitespace-nowrap"
                                          >
                                            Del
                                          </button>
                                        </div>
                                      </div>
                                    </>
                                  )}
                                </div>
                              )
                            })}
                          </div>
                        </td>
                      </tr>
                    )}
                  </React.Fragment>
                )
              })
            )}
          </tbody>
        </table>
      </div>

      {sellingHolding && (
        <SellModal
          holding={sellingHolding}
          onConfirm={handleSellConfirm}
          onClose={() => setSellingHolding(null)}
        />
      )}

      {deletingHolding && (
        <ConfirmDialog
          title="删除资产"
          message={`确定删除 ${deletingHolding.name || deletingHolding.symbol || "此资产"}？此操作不可撤销。`}
          onConfirm={() => {
            onRemoveHolding(deletingHolding.id)
            setDeletingHolding(null)
          }}
          onCancel={() => setDeletingHolding(null)}
        />
      )}
    </div>
  )
}
