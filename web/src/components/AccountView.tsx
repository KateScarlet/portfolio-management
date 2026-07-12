import { useState, useEffect, useCallback } from "react"
import React from "react"
import Decimal from "decimal.js"
import {
  Account,
  ASSET_DEFINITIONS,
  Holding,
  HoldingWithAccount,
  Portfolio,
  ColorScheme,
} from "../types"
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
import ConfirmDialog from "./ConfirmDialog"
import AssetIcon from "./AssetIcon"

interface Props {
  selectedAccount: Account | null
  colorScheme: ColorScheme
  portfolios: Portfolio[]
  currentPortfolio: Portfolio | null
  accounts: Account[]
  onAddHolding: (holding: Omit<Holding, "id">) => Promise<void>
  onRefreshAvailableFunds: () => Promise<void>
}

export default function AccountView({
  selectedAccount,
  colorScheme,
  portfolios: _portfolios,
  currentPortfolio,
  accounts,
  onAddHolding,
  onRefreshAvailableFunds,
}: Props) {
  const [holdings, setHoldings] = useState<HoldingWithAccount[] | null>(null)
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [isAdding, setIsAdding] = useState(false)
  const [buyingHolding, setBuyingHolding] = useState<HoldingWithAccount | null>(null)
  const [sellingHolding, setSellingHolding] = useState<HoldingWithAccount | null>(null)
  const [deletingHolding, setDeletingHolding] = useState<HoldingWithAccount | null>(null)

  const selectedAccountId = selectedAccount?.id

  const loadHoldings = useCallback(async () => {
    try {
      const data = await api.fetchAccountViewHoldings(undefined, selectedAccountId)
      setHoldings(data)
    } catch (e) {
      console.error("Failed to load account holdings", e)
      setHoldings([])
    }
  }, [selectedAccountId])

  useEffect(() => {
    let cancelled = false
    api
      .fetchAccountViewHoldings(undefined, selectedAccountId)
      .then((data) => {
        if (!cancelled) setHoldings(data)
      })
      .catch(() => {
        if (!cancelled) setHoldings([])
      })
    return () => {
      cancelled = true
    }
  }, [selectedAccountId])

  const handleAddHolding = useCallback(
    async (h: Omit<Holding, "id">) => {
      await onAddHolding(h)
      await loadHoldings()
      onRefreshAvailableFunds()
    },
    [onAddHolding, loadHoldings, onRefreshAvailableFunds]
  )

  const handleBuyConfirm = useCallback(async () => {
    await loadHoldings()
    onRefreshAvailableFunds()
  }, [loadHoldings, onRefreshAvailableFunds])

  const handleSellConfirm = useCallback(async () => {
    await loadHoldings()
    onRefreshAvailableFunds()
  }, [loadHoldings, onRefreshAvailableFunds])

  // Group holdings by account when viewing all accounts
  const showGrouped = !selectedAccount && holdings && holdings.length > 0
  const accountGroups = showGrouped
    ? (() => {
        const groups = new Map<string, HoldingWithAccount[]>()
        for (const h of holdings) {
          const key = h.accountName || "未分配"
          const list = groups.get(key)
          if (list) list.push(h)
          else groups.set(key, [h])
        }
        return Array.from(groups.entries())
      })()
    : []

  const renderHoldingRow = (h: HoldingWithAccount) => {
    const def = ASSET_DEFINITIONS[h.assetId]
    const isExpanded = expandedId === h.id
    const value = toDecimal(h.value)
    const cost = toDecimal(h.cost)
    const profit = value.minus(cost)
    const returnRate = cost.isZero() ? new Decimal(0) : profit.div(cost)
    const profitColor = getProfitColor(profit.isPositive(), colorScheme)

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
                  <p className="text-[10px] text-[#ADB5BD] truncate max-w-37.5" title={h.name}>
                    {h.name}
                  </p>
                  <span className="text-[10px] bg-blue-50 text-blue-600 px-1.5 py-0.5 rounded">
                    {h.currency || "CNY"}
                  </span>
                </div>
              </div>
            ) : (
              <div className="flex items-center gap-2">
                <p className="text-sm font-mono">{h.name || "手工资产"}</p>
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
          {!selectedAccount && (
            <td className="px-6 py-5">
              <span className="text-xs text-[#6C757D] bg-[#F8F9FA] px-2 py-1 rounded">
                {h.accountName || "未分配"}
              </span>
            </td>
          )}
          <td className="px-6 py-5 text-right font-mono text-sm text-[#495057]">
            {toDecimal(h.shares).isPositive() ? (
              <div>
                <p>
                  {formatPrice(toDecimal(h.value).div(h.shares).toString(), h.currency || "CNY")}
                </p>
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
                    含分红 {formatCurrencyByCode(h.totalDividends || "0", h.currency || "CNY")}
                  </p>
                )}
              </div>
            ) : (
              <span className="text-[#ADB5BD] text-xs">-</span>
            )}
          </td>
          <td className="px-6 py-5 text-right font-mono text-sm text-[#495057]">
            {h.cost && toDecimal(h.cost).isPositive() ? (
              <p className={profitColor}>
                {profit.isPositive() ? "+" : ""}
                {formatPercent(returnRate.toNumber())}
              </p>
            ) : (
              <span className="text-[#ADB5BD] text-xs">-</span>
            )}
          </td>
          <td className="px-6 py-5 text-right font-medium text-sm font-mono">
            {formatCurrencyByCode(h.value, h.currency || "CNY")}
          </td>
          <td className="px-6 py-5 text-right" onClick={(e) => e.stopPropagation()}>
            <div className="flex gap-2 justify-end">
              <button
                onClick={() => setBuyingHolding(h)}
                className="text-[10px] uppercase tracking-wider text-[#1A1A1A] hover:text-emerald-600 font-bold transition-colors"
              >
                Buy
              </button>
              <button
                onClick={() => setSellingHolding(h)}
                className="text-[10px] uppercase tracking-wider text-[#1A1A1A] hover:text-orange-500 font-bold transition-colors"
              >
                Sell
              </button>
              <button
                onClick={() => setDeletingHolding(h)}
                className="text-[10px] uppercase tracking-wider text-[#ADB5BD] hover:text-orange-500 font-bold transition-colors"
              >
                Del
              </button>
            </div>
          </td>
        </tr>
        {isExpanded && h.lots && h.lots.length > 0 && (
          <tr>
            <td colSpan={selectedAccount ? 7 : 8} className="px-6 py-3 bg-[#F8F9FA]">
              <div className="text-xs">
                <div className="font-medium text-[#6C757D] mb-2">交易记录</div>
                <table className="w-full">
                  <thead>
                    <tr className="text-[10px] text-[#ADB5BD]">
                      <th className="text-left pb-1">日期</th>
                      <th className="text-right pb-1">类型</th>
                      <th className="text-right pb-1">份额</th>
                      <th className="text-right pb-1">成本价</th>
                      <th className="text-right pb-1">成本</th>
                      <th className="text-right pb-1">手续费</th>
                    </tr>
                  </thead>
                  <tbody>
                    {h.lots.map((lot) => (
                      <tr key={lot.id} className="border-t border-[#E9ECEF]">
                        <td className="py-1.5 text-[#495057]">
                          {lot.date ? new Date(lot.date).toLocaleDateString() : "-"}
                        </td>
                        <td className="py-1.5 text-right">
                          <span
                            className={`px-1.5 py-0.5 rounded text-[10px] ${lot.type === "sell" ? "bg-red-50 text-red-600" : "bg-emerald-50 text-emerald-600"}`}
                          >
                            {lot.type === "sell" ? "卖出" : "买入"}
                          </span>
                        </td>
                        <td className="py-1.5 text-right font-mono">{lot.shares}</td>
                        <td className="py-1.5 text-right font-mono">
                          {lot.costPrice ? formatPrice(lot.costPrice, h.currency || "CNY") : "-"}
                        </td>
                        <td className="py-1.5 text-right font-mono">
                          {lot.cost ? formatCurrencyByCode(lot.cost, h.currency || "CNY") : "-"}
                        </td>
                        <td className="py-1.5 text-right font-mono text-[#6C757D]">
                          {lot.fee ? formatCurrencyByCode(lot.fee, h.currency || "CNY") : "-"}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </td>
          </tr>
        )}
      </React.Fragment>
    )
  }

  const colSpan = selectedAccount ? 7 : 8

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <h2 className="text-lg font-semibold">账户视图</h2>
          {selectedAccount && (
            <span className="text-xs text-[#6C757D] bg-white border border-[#E9ECEF] rounded px-2 py-1">
              {selectedAccount.name}
            </span>
          )}
        </div>
      </div>

      <div className="bg-white rounded-2xl border border-[#E9ECEF] shadow-sm flex flex-col overflow-hidden">
        <div className="p-6 border-b border-[#F1F3F5] flex justify-between items-center bg-white flex-wrap gap-4">
          <h3 className="text-lg font-medium text-[#1A1A1A]">持仓明细</h3>
          <div className="flex gap-2">
            <button
              onClick={() => setIsAdding(!isAdding)}
              className="text-xs bg-[#1A1A1A] text-white px-4 py-2 rounded-full hover:opacity-90 transition-opacity"
            >
              {isAdding ? "取消" : "+ 录入资产"}
            </button>
          </div>
        </div>

        {isAdding && currentPortfolio && (
          <AddHoldingForm
            onAddHolding={handleAddHolding}
            onClose={() => setIsAdding(false)}
            accounts={accounts}
            accountId={selectedAccount?.id}
          />
        )}

        {holdings === null ? (
          <div className="p-8 text-center">
            <p className="text-sm text-[#6C757D]">加载中...</p>
          </div>
        ) : (
          <div className="grow overflow-x-auto">
            <table className="w-full text-left">
              <thead className="text-[10px] uppercase tracking-widest text-[#ADB5BD] border-b border-[#F1F3F5] bg-white">
                <tr>
                  <th className="px-6 py-4 font-bold">资产大类</th>
                  <th className="px-6 py-4 font-bold">代码/名称</th>
                  {!selectedAccount && <th className="px-6 py-4 font-bold">账户</th>}
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
                    <td colSpan={colSpan} className="px-6 py-12 text-center text-sm text-[#ADB5BD]">
                      暂无持仓数据，点击"录入资产"开始
                    </td>
                  </tr>
                ) : showGrouped ? (
                  accountGroups.map(([accountName, groupHoldings]) => (
                    <React.Fragment key={accountName}>
                      <tr>
                        <td colSpan={colSpan} className="px-6 py-3 bg-[#F8F9FA]">
                          <div className="flex items-center justify-between">
                            <span className="text-xs font-semibold text-[#1A1A1A]">
                              {accountName}
                            </span>
                            <span className="text-[10px] text-[#6C757D]">
                              {groupHoldings.length} 个持仓
                            </span>
                          </div>
                        </td>
                      </tr>
                      {groupHoldings.map((h) => renderHoldingRow(h))}
                    </React.Fragment>
                  ))
                ) : (
                  holdings.map((h) => renderHoldingRow(h))
                )}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {buyingHolding && currentPortfolio && buyingHolding.portfolioId && (
        <BuyModal
          portfolioId={buyingHolding.portfolioId}
          holding={buyingHolding}
          onConfirm={handleBuyConfirm}
          onClose={() => setBuyingHolding(null)}
          accounts={accounts}
          accountId={selectedAccount?.id}
        />
      )}
      {sellingHolding && currentPortfolio && sellingHolding.portfolioId && (
        <SellModal
          portfolioId={sellingHolding.portfolioId}
          holding={sellingHolding}
          displayCurrency={sellingHolding.currency || "CNY"}
          onConfirm={handleSellConfirm}
          onClose={() => setSellingHolding(null)}
          accounts={accounts}
        />
      )}
      {deletingHolding && deletingHolding.portfolioId && (
        <ConfirmDialog
          title="删除资产"
          message={`确定删除 ${deletingHolding.name || deletingHolding.symbol || "此资产"}？此操作不可撤销。`}
          onConfirm={async () => {
            try {
              await api.deleteHolding(deletingHolding.portfolioId!, deletingHolding.id)
              await loadHoldings()
              onRefreshAvailableFunds()
            } catch (e) {
              console.error("Failed to delete holding", e)
            }
            setDeletingHolding(null)
          }}
          onCancel={() => setDeletingHolding(null)}
        />
      )}
    </div>
  )
}
