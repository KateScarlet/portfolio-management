import React, { useState, useCallback, useEffect } from "react"
import {
  Account,
  ASSET_DEFINITIONS,
  Dividend,
  Holding,
  HoldingLot,
  MergedHolding,
  ColorScheme,
} from "../types"
import AssetIcon from "./AssetIcon"
import {
  formatCurrencyByCode,
  formatPrice,
  formatPercent,
  getProfitColor,
  toDecimal,
} from "../utils"
import * as api from "../api"
import AddHoldingForm from "./AddHoldingForm"
import BuyModal from "./BuyModal"
import SellModal from "./SellModal"
import DividendModal from "./DividendModal"
import ConfirmDialog from "./ConfirmDialog"
import { useToast } from "./toast-context"

interface HoldingsManagerProps {
  portfolioId: string
  holdings: MergedHolding[]
  setHoldings: React.Dispatch<React.SetStateAction<MergedHolding[]>>
  total: string
  onAddHolding: (holding: Omit<Holding, "id">) => Promise<void>
  onUpdateHolding: (id: string, updates: Partial<Holding>) => void
  onRemoveHolding: (id: string) => void
  onSaveRecord: () => void
  colorScheme: ColorScheme
  displayCurrency: string
  onRefreshAvailableFunds: () => Promise<void>
  onSyncComplete: (status: { lastSyncAt: string; lastSyncErr?: string; syncing: boolean }) => void
  accounts?: Account[]
}

export default function HoldingsManager({
  portfolioId,
  holdings,
  setHoldings,
  total: _total,
  onAddHolding,
  onUpdateHolding,
  onRemoveHolding,
  onSaveRecord,
  colorScheme,
  displayCurrency,
  onRefreshAvailableFunds,
  onSyncComplete,
  accounts = [],
}: HoldingsManagerProps) {
  const { showToast } = useToast()
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
  const [sellingHolding, setSellingHolding] = useState<Holding | null>(null)
  const [buyingHolding, setBuyingHolding] = useState<Holding | null>(null)
  const [deletingHolding, setDeletingHolding] = useState<Holding | null>(null)
  const [dividendHolding, setDividendHolding] = useState<Holding | null>(null)
  const [expandedDividends, setExpandedDividends] = useState<Dividend[]>([])
  const [editingDividend, setEditingDividend] = useState<Dividend | null>(null)
  const [deletingDividend, setDeletingDividend] = useState<Dividend | null>(null)

  useEffect(() => {
    if (!expandedId) return
    let cancelled = false
    api
      .fetchDividends(portfolioId, expandedId)
      .then((data) => {
        if (!cancelled) setExpandedDividends(data)
      })
      .catch(() => {
        if (!cancelled) setExpandedDividends([])
      })
    return () => {
      cancelled = true
    }
  }, [expandedId, portfolioId])

  const computeCost = useCallback((costPriceStr: string, sharesStr: string) => {
    const p = toDecimal(costPriceStr)
    const s = toDecimal(sharesStr)
    return p.times(s).toString()
  }, [])

  const syncAllPrices = useCallback(async () => {
    setSyncing(true)
    try {
      const status = await api.triggerSync(portfolioId)
      onSyncComplete(status)
      const freshHoldings = await api.fetchHoldings(portfolioId)
      setHoldings(freshHoldings)
    } finally {
      setSyncing(false)
    }
  }, [onSyncComplete, setHoldings, portfolioId])

  const saveEditLot = useCallback(
    async (targetHoldingId: string, h: Holding, lotId: string, updatedFields: Partial<HoldingLot>) => {
      try {
        await api.updateLot(portfolioId, targetHoldingId, lotId, updatedFields)
        const freshHoldings = await api.fetchHoldings(portfolioId)
        setHoldings(freshHoldings)
      } catch (e) {
        showToast(e instanceof Error ? e.message : "更新交易记录失败", "error")
      }
      setEditingLotId(null)
    },
    [portfolioId, setHoldings, showToast]
  )

  const deleteEditLot = useCallback(
    async (targetHoldingId: string, h: Holding, lotId: string) => {
      try {
        await api.deleteLot(portfolioId, targetHoldingId, lotId)
        const freshHoldings = await api.fetchHoldings(portfolioId)
        setHoldings(freshHoldings)
      } catch (e) {
        showToast(e instanceof Error ? e.message : "删除交易记录失败", "error")
      }
      setEditingLotId(null)
      onRefreshAvailableFunds()
    },
    [portfolioId, setHoldings, onRefreshAvailableFunds, showToast]
  )

  const handleSellConfirm = useCallback(async () => {
    try {
      const freshHoldings = await api.fetchHoldings(portfolioId)
      setHoldings(freshHoldings)
    } catch (e) {
      showToast(e instanceof Error ? e.message : "刷新持仓失败", "error")
    }
    onRefreshAvailableFunds()
  }, [portfolioId, setHoldings, onRefreshAvailableFunds, showToast])

  const handleBuyConfirm = useCallback(async () => {
    try {
      const freshHoldings = await api.fetchHoldings(portfolioId)
      setHoldings(freshHoldings)
    } catch (e) {
      showToast(e instanceof Error ? e.message : "刷新持仓失败", "error")
    }
    onRefreshAvailableFunds()
  }, [portfolioId, setHoldings, onRefreshAvailableFunds, showToast])

  const handleDividendConfirm = useCallback(async () => {
    try {
      const freshHoldings = await api.fetchHoldings(portfolioId)
      setHoldings(freshHoldings)
    } catch (e) {
      showToast(e instanceof Error ? e.message : "刷新持仓失败", "error")
    }
    onRefreshAvailableFunds()
  }, [portfolioId, setHoldings, onRefreshAvailableFunds, showToast])

  // Pre-compute lot groups for each holding (for merged view)
  const lotGroupsByHolding = new Map<
    string,
    { name: string; lots: HoldingLot[]; holdingId: string }[]
  >()
  for (const h of holdings) {
    const accs = ("accounts" in h ? (h as MergedHolding).accounts : null) || []
    if (accs.length > 1) {
      lotGroupsByHolding.set(
        h.id,
        accs.map((acc) => ({
          name: acc.accountName || "未分配",
          lots: acc.lots || [],
          holdingId: acc.holdingId,
        }))
      )
    } else {
      lotGroupsByHolding.set(h.id, [{ name: "", lots: h.lots || [], holdingId: h.id }])
    }
  }

  const renderLot = (h: MergedHolding, lot: HoldingLot, holdingId: string) => {
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
                value={lot.date ? lot.date.split("T")[0] : ""}
                onChange={(e) => {
                  saveEditLot(holdingId, h, lot.id, {
                    date: e.target.value ? new Date(e.target.value).toISOString() : lot.date,
                  })
                }}
                className="px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A]"
              />
            </div>
            {h.symbol && (
              <>
                <div className="flex flex-col gap-0.5">
                  <span className="text-[9px] text-[#ADB5BD] uppercase">单价</span>
                  <input
                    type="number"
                    placeholder="0"
                    value={editingLotCostPrice}
                    onChange={(e) => {
                      setEditingLotCostPrice(e.target.value)
                      setEditingLotCost(computeCost(e.target.value, editingLotShares))
                    }}
                    className="w-24 px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A] font-mono"
                  />
                </div>
                <div className="flex flex-col gap-0.5">
                  <span className="text-[9px] text-[#ADB5BD] uppercase">数量</span>
                  <input
                    type="number"
                    placeholder="0"
                    value={editingLotShares}
                    onChange={(e) => {
                      setEditingLotShares(e.target.value)
                      setEditingLotCost(computeCost(editingLotCostPrice, e.target.value))
                    }}
                    className="w-20 px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A] font-mono"
                  />
                </div>
              </>
            )}
            {h.symbol ? (
              <div className="flex flex-col gap-0.5">
                <span className="text-[9px] text-[#ADB5BD] uppercase">成本</span>
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
                className="w-20 px-2 py-1 border border-[#E9ECEF] rounded text-xs font-mono focus:outline-none focus:border-[#1A1A1A]"
              />
            </div>
            <div className="flex gap-2 shrink-0">
              <button
                onClick={() =>
                  saveEditLot(holdingId, h, lot.id, {
                    costPrice: h.symbol ? editingLotCostPrice : undefined,
                    shares: editingLotShares,
                    cost: editingLotCost,
                    valueAdded: !h.symbol ? editingLotValue : undefined,
                    fee: editingLotFee,
                  })
                }
                className="text-[10px] uppercase tracking-wider text-[#1A1A1A] hover:text-blue-600 font-bold transition-colors"
              >
                Save
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
                    单价: {formatPrice(lot.costPrice || 0, h.currency || "CNY")}
                  </span>
                  <span className="w-20 text-right">×{lot.shares}</span>
                </>
              ) : (
                <>
                  <span className="w-20 text-right">{lot.shares}份</span>
                  <span className="w-28 text-right">
                    成本: {formatCurrencyByCode(lot.cost || 0, h.currency || "CNY")}
                  </span>
                  <span className="w-28 text-right">
                    价值: {formatCurrencyByCode(lot.valueAdded || 0, h.currency || "CNY")}
                  </span>
                </>
              )}
              {toDecimal(lot.fee).isPositive() && (
                <span className="w-20 text-[10px] text-[#ADB5BD] text-right">
                  手续费: {formatCurrencyByCode(lot.fee || 0, h.currency || "CNY")}
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
                  }}
                  className="text-[10px] uppercase tracking-wider text-[#1A1A1A] hover:text-blue-600 font-bold transition-colors whitespace-nowrap"
                >
                  Edit
                </button>
                <button
                  onClick={() => deleteEditLot(holdingId, h, lot.id)}
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
  }

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
        <AddHoldingForm onAddHolding={onAddHolding} onClose={() => setIsAdding(false)} accounts={accounts} />
      )}

      <div className="grow overflow-x-auto">
        <table className="w-full text-left">
          <thead className="text-[10px] uppercase tracking-widest text-[#ADB5BD] border-b border-[#F1F3F5] bg-white">
            <tr>
              <th className="px-6 py-4 font-bold">资产大类</th>
              <th className="px-6 py-4 font-bold">代码/名称</th>
              <th className="px-6 py-4 font-bold text-right">净值 & 份额</th>
              <th className="px-6 py-4 font-bold text-right">总成本</th>
              <th className="px-6 py-4 font-bold text-right">盈亏</th>
              <th className="px-6 py-4 font-bold text-right">当前总市值</th>
              <th className="px-6 py-4 font-bold text-right">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[#F8F9FA] bg-white text-[#1A1A1A]">
            {holdings.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-6 py-12 text-center text-sm text-[#ADB5BD]">
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
                        <AssetIcon assetId={h.assetId} />
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
                            <div className="flex items-center gap-2 mt-0.5">
                              <p
                                className="text-[10px] text-[#ADB5BD] truncate max-w-37.5"
                                title={h.name}
                              >
                                {h.name}
                              </p>
                              <span className="text-[10px] bg-blue-50 text-blue-600 px-1.5 py-0.5 rounded">
                                {h.currency || "CNY"}
                              </span>
                            </div>
                          </div>
                        ) : (
                          <div className="flex items-center gap-2">
                            <p className="text-sm text-[#6C757D]">{h.name || "手工资产"}</p>
                            {h.lots && h.lots.length > 0 && (
                              <span className="text-[10px] bg-gray-100 text-gray-500 px-1.5 py-0.5 rounded">
                                {h.lots.length} 笔
                              </span>
                            )}
                            <span className="text-[10px] bg-blue-50 text-blue-600 px-1.5 py-0.5 rounded">
                              {h.currency || "CNY"}
                            </span>
                          </div>
                        )}
                      </td>
                      <td className="px-6 py-5 text-right font-mono text-sm text-[#495057]">
                        {h.symbol ? (
                          <div>
                            <p>{formatPrice(h.price, h.currency || "CNY")}</p>
                            <p className="text-[10px] text-[#ADB5BD]">× {h.shares}</p>
                          </div>
                        ) : toDecimal(h.shares).isPositive() ? (
                          <div>
                            {toDecimal(h.costPrice).isPositive() && (
                              <p>{formatPrice(h.costPrice, h.currency || "CNY")}</p>
                            )}
                            <p className="text-[10px] text-[#ADB5BD]">× {h.shares}</p>
                          </div>
                        ) : (
                          <span className="text-[#ADB5BD] text-xs">-</span>
                        )}
                      </td>
                      <td className="px-6 py-5 text-right font-mono text-sm text-[#495057]">
                        {h.cost && toDecimal(h.cost).isPositive() ? (
                          <div>
                            <p>{formatCurrencyByCode(h.cost, h.currency || "CNY")}</p>
                            {toDecimal(h.totalDividends || "0").isPositive() && (
                              <p className="text-[10px] text-yellow-600">
                                含分红{" "}
                                {formatCurrencyByCode(
                                  h.totalDividends || "0",
                                  h.currency || "CNY"
                                )}
                              </p>
                            )}
                          </div>
                        ) : (
                          <span className="text-[#ADB5BD] text-xs">-</span>
                        )}
                      </td>
                      <td className="px-6 py-5 text-right font-mono text-sm text-[#495057]">
                        {(() => {
                          const value = toDecimal(h.value)
                          const cost = toDecimal(h.cost)
                          const totalDividends = toDecimal(h.totalDividends || "0")
                          if (
                            cost.isZero() ||
                            !Number.isFinite(value.toNumber()) ||
                            !Number.isFinite(cost.toNumber())
                          ) {
                            return <span className="text-[#ADB5BD] text-xs">-</span>
                          }
                          const profit = value.minus(cost).plus(totalDividends)
                          const returnRate = profit.div(cost)
                          const rateNum = returnRate.toNumber()
                          if (!Number.isFinite(rateNum)) {
                            return <span className="text-[#ADB5BD] text-xs">-</span>
                          }
                          const isPositive = profit.isPositive()
                          return (
                            <p
                              className={getProfitColor(isPositive, colorScheme)}
                            >
                              {isPositive ? "+" : ""}
                              {formatPercent(rateNum)}
                            </p>
                          )
                        })()}
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
                                const val = toDecimal(tempEditValue)
                                if (!val.isNegative()) {
                                  onUpdateHolding(h.id, { value: val.toString() })
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
                          formatCurrencyByCode(h.value, h.currency || "CNY")
                        )}
                      </td>
                      <td
                        className="px-6 py-5 text-right space-x-2"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <button
                          onClick={() => setBuyingHolding(h)}
                          className="text-[10px] uppercase tracking-wider text-[#1A1A1A] hover:text-green-600 font-bold transition-colors"
                          title="买入"
                        >
                          Buy
                        </button>
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
                        <button
                          onClick={() => setSellingHolding(h)}
                          className="text-[10px] uppercase tracking-wider text-[#1A1A1A] hover:text-orange-500 font-bold transition-colors"
                          title="卖出"
                        >
                          Sell
                        </button>
                        <button
                          onClick={() => setDividendHolding(h)}
                          className="text-[10px] uppercase tracking-wider text-[#1A1A1A] hover:text-yellow-600 font-bold transition-colors"
                          title="分红"
                        >
                          Div
                        </button>
                        <button
                          onClick={() => setDeletingHolding(h)}
                          className="text-[10px] uppercase tracking-wider text-[#ADB5BD] hover:text-orange-500 font-bold transition-colors"
                        >
                          Del
                        </button>
                      </td>
                    </tr>

                    {isExpanded &&
                      (() => {
                        const groups = lotGroupsByHolding.get(h.id) || []
                        const hasMulti = groups.length > 1
                        return (
                          <tr className="bg-[#F8F9FA]">
                            <td colSpan={6} className="px-6 py-4">
                              <div className="pl-6 border-l-2 border-[#DEE2E6] ml-4 space-y-3">
                                {groups.map((group, gi) => (
                                  <div key={gi} className="space-y-1">
                                    {hasMulti && (
                                      <div className="text-xs font-medium text-[#1A1A1A] mb-1">
                                        {group.name}
                                      </div>
                                    )}
                                    {group.lots.map((lot) => renderLot(h, lot, group.holdingId))}
                                  </div>
                                ))}
                                {expandedDividends.length > 0 && (
                                  <div className="mt-3 pt-3 border-t border-[#DEE2E6]">
                                    <div className="text-xs font-medium text-[#6C757D] mb-2">
                                      分红记录
                                    </div>
                                    {expandedDividends.map((div) => (
                                      <div
                                        key={div.id}
                                        className="flex justify-between items-center text-xs font-mono pb-2"
                                      >
                                        <span className="flex items-center gap-2">
                                          <span className="text-[9px] bg-yellow-100 text-yellow-600 px-1.5 py-0.5 rounded font-bold">
                                            {div.reinvest ? "再投资" : "分红"}
                                          </span>
                                          {new Date(div.createdAt).toLocaleDateString()}
                                        </span>
                                        <div className="flex items-center gap-2 text-right">
                                          <span className="w-24 text-right text-yellow-600">
                                            +
                                            {formatCurrencyByCode(
                                              div.netAmount,
                                              div.currency || "CNY"
                                            )}
                                          </span>
                                          {div.reinvest && div.reinvestShares && (
                                            <span className="w-20 text-right text-[#6C757D]">
                                              ×{div.reinvestShares}
                                            </span>
                                          )}
                                          <button
                                            onClick={() => setEditingDividend(div)}
                                            className="text-[10px] text-[#ADB5BD] hover:text-[#495057] transition-colors"
                                          >
                                            编辑
                                          </button>
                                          <button
                                            onClick={() => setDeletingDividend(div)}
                                            className="text-[10px] text-[#ADB5BD] hover:text-red-500 transition-colors"
                                          >
                                            删除
                                          </button>
                                        </div>
                                      </div>
                                    ))}
                                  </div>
                                )}
                              </div>
                            </td>
                          </tr>
                        )
                      })()}
                  </React.Fragment>
                )
              })
            )}
          </tbody>
        </table>
      </div>

      {sellingHolding && (
        <SellModal
          portfolioId={portfolioId}
          holding={sellingHolding}
          displayCurrency={displayCurrency}
          onConfirm={handleSellConfirm}
          onClose={() => setSellingHolding(null)}
          accounts={accounts}
        />
      )}

      {buyingHolding && (
        <BuyModal
          portfolioId={portfolioId}
          holding={buyingHolding}
          onConfirm={handleBuyConfirm}
          onClose={() => setBuyingHolding(null)}
          accounts={accounts}
        />
      )}

      {dividendHolding && (
        <DividendModal
          portfolioId={portfolioId}
          holding={dividendHolding}
          onConfirm={handleDividendConfirm}
          onClose={() => setDividendHolding(null)}
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

      {deletingDividend && (
        <ConfirmDialog
          title="删除分红记录"
          message={`确定删除此分红记录？${deletingDividend.reinvest ? "再投资份额将被撤销。" : "可用资金将被扣除。"}此操作不可撤销。`}
          onConfirm={async () => {
            try {
              await api.deleteDividend(portfolioId, deletingDividend.id)
              setExpandedDividends((prev) => prev.filter((d) => d.id !== deletingDividend.id))
              const freshHoldings = await api.fetchHoldings(portfolioId)
              setHoldings(freshHoldings)
              onRefreshAvailableFunds()
            } catch {
              // error handled by api layer
            }
            setDeletingDividend(null)
          }}
          onCancel={() => setDeletingDividend(null)}
        />
      )}

      {editingDividend &&
        (() => {
          const h = holdings.find((h) => h.id === editingDividend.holdingId)
          if (!h) return null
          return (
            <DividendModal
              portfolioId={portfolioId}
              holding={h}
              dividend={editingDividend}
              onConfirm={async () => {
                setEditingDividend(null)
                const freshDividends = await api.fetchDividends(
                  portfolioId,
                  editingDividend.holdingId
                )
                setExpandedDividends(freshDividends)
                const freshHoldings = await api.fetchHoldings(portfolioId)
                setHoldings(freshHoldings)
                onRefreshAvailableFunds()
              }}
              onClose={() => setEditingDividend(null)}
            />
          )
        })()}
    </div>
  )
}
