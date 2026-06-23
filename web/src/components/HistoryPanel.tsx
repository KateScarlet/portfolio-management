import React, { useState } from "react"
import { format } from "date-fns"
import { PortfolioRecord, ColorScheme } from "../types"
import { formatCurrency, formatPercent, getProfitColor } from "../utils"
import ConfirmDialog from "./ConfirmDialog"

interface HistoryPanelProps {
  history: PortfolioRecord[]
  onDeleteRecord: (id: string) => void
  colorScheme: ColorScheme
}

export default function HistoryPanel({ history, onDeleteRecord, colorScheme }: HistoryPanelProps) {
  const [deletingRecord, setDeletingRecord] = useState<PortfolioRecord | null>(null)
  const [expandedId, setExpandedId] = useState<string | null>(null)

  if (history.length === 0) {
    return null
  }

  return (
    <div className="bg-white rounded-2xl border border-[#E9ECEF] shadow-sm flex flex-col overflow-hidden">
      <div className="p-6 border-b border-[#F1F3F5]">
        <h3 className="text-lg font-medium text-[#1A1A1A]">历史快照与盈亏记录</h3>
      </div>

      <div className="grow overflow-x-auto">
        <table className="w-full text-left">
          <thead className="text-[10px] uppercase tracking-widest text-[#ADB5BD] border-b border-[#F1F3F5] bg-white">
            <tr>
              <th className="px-6 py-4 font-bold">日期</th>
              <th className="px-6 py-4 font-bold">总价值</th>
              <th className="px-6 py-4 font-bold">总成本</th>
              <th className="px-6 py-4 font-bold">累计盈亏</th>
              <th className="px-6 py-4 font-bold">股票</th>
              <th className="px-6 py-4 font-bold">债券</th>
              <th className="px-6 py-4 font-bold">商品</th>
              <th className="px-6 py-4 font-bold">现金</th>
              <th className="px-6 py-4 font-bold text-right">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[#F8F9FA] bg-white text-[#1A1A1A]">
            {history.map((record) => {
              const principal = record.principal
              const profit = record.total - principal
              const returnRate = principal > 0 ? profit / principal : 0
              const isPositive = profit >= 0
              const isExpanded = expandedId === record.id

              return (
                <React.Fragment key={record.id}>
                  <tr
                    className="hover:bg-[#F8F9FA] transition-colors cursor-pointer"
                    onClick={() => setExpandedId(isExpanded ? null : record.id)}
                  >
                    <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-[#6C757D]">
                      <span className="mr-1 inline-block w-3 text-center text-[10px]">
                        {isExpanded ? "▾" : "▸"}
                      </span>
                      {format(record.timestamp, "MM/dd, HH:mm")}
                    </td>
                    <td className="px-6 py-4 font-mono text-sm font-medium">
                      {formatCurrency(record.total)}
                    </td>
                    <td className="px-6 py-4 font-mono text-sm text-[#495057]">
                      {principal > 0 ? formatCurrency(principal) : "-"}
                    </td>
                    <td className="px-6 py-4 font-mono text-sm">
                      {principal > 0 ? (
                        <span className={getProfitColor(isPositive, colorScheme)}>
                          {isPositive ? "+" : ""}
                          {formatCurrency(profit)}
                          <br />
                          <span className="text-[10px] opacity-80">
                            {isPositive ? "+" : ""}
                            {formatPercent(returnRate)}
                          </span>
                        </span>
                      ) : (
                        <span className="text-[#ADB5BD]">-</span>
                      )}
                    </td>
                    <td className="px-6 py-4 text-xs font-mono text-[#ADB5BD]">
                      {formatCurrency(record.assets.stocks || 0)}
                    </td>
                    <td className="px-6 py-4 text-xs font-mono text-[#ADB5BD]">
                      {formatCurrency(record.assets.bonds || 0)}
                    </td>
                    <td className="px-6 py-4 text-xs font-mono text-[#ADB5BD]">
                      {formatCurrency(record.assets.commodities || 0)}
                    </td>
                    <td className="px-6 py-4 text-xs font-mono text-[#ADB5BD]">
                      {formatCurrency(record.assets.cash || 0)}
                    </td>
                    <td className="px-6 py-4 text-right">
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          setDeletingRecord(record)
                        }}
                        className="text-[10px] uppercase tracking-wider text-[#ADB5BD] hover:text-orange-500 font-bold transition-colors"
                        title="删除记录"
                      >
                        Del
                      </button>
                    </td>
                  </tr>
                  {isExpanded && record.holdings.length > 0 && (
                    <tr className="bg-[#F8F9FA]">
                      <td colSpan={9} className="px-6 py-4">
                        <div className="pl-12 border-l-2 border-[#DEE2E6] ml-4">
                          <h5 className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold mb-2">
                            持仓明细
                          </h5>
                          <table className="w-full text-xs">
                            <thead>
                              <tr className="text-[10px] uppercase tracking-widest text-[#ADB5BD]">
                                <th className="text-left pb-1">代码</th>
                                <th className="text-left pb-1">名称</th>
                                <th className="text-left pb-1">类别</th>
                                <th className="text-right pb-1">持仓</th>
                                <th className="text-right pb-1">价格</th>
                                <th className="text-right pb-1">市值</th>
                                <th className="text-right pb-1">成本</th>
                                <th className="text-right pb-1">盈亏</th>
                              </tr>
                            </thead>
                            <tbody className="font-mono">
                              {record.holdings.map((h, i) => {
                                const pnl = h.value - h.cost
                                return (
                                  <tr key={i} className="border-b border-[#E9ECEF] last:border-0">
                                    <td className="py-1.5">{h.symbol || "-"}</td>
                                    <td className="py-1.5 text-[#6C757D]">{h.name}</td>
                                    <td className="py-1.5 text-[#6C757D]">{h.assetId}</td>
                                    <td className="py-1.5 text-right">{h.shares.toFixed(2)}</td>
                                    <td className="py-1.5 text-right">{formatCurrency(h.price)}</td>
                                    <td className="py-1.5 text-right">{formatCurrency(h.value)}</td>
                                    <td className="py-1.5 text-right">{formatCurrency(h.cost)}</td>
                                    <td className={`py-1.5 text-right ${getProfitColor(pnl >= 0, colorScheme)}`}>
                                      {pnl >= 0 ? "+" : ""}{formatCurrency(pnl)}
                                    </td>
                                  </tr>
                                )
                              })}
                            </tbody>
                          </table>
                        </div>
                      </td>
                    </tr>
                  )}
                </React.Fragment>
              )
            })}
          </tbody>
        </table>
      </div>

      {deletingRecord && (
        <ConfirmDialog
          title="删除记录"
          message={`确定删除 ${format(deletingRecord.timestamp, "MM/dd HH:mm")} 的快照记录？此操作不可撤销。`}
          onConfirm={() => {
            onDeleteRecord(deletingRecord.id)
            setDeletingRecord(null)
          }}
          onCancel={() => setDeletingRecord(null)}
        />
      )}
    </div>
  )
}
