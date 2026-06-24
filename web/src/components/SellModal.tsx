import { useState } from "react"
import { Holding } from "../types"
import { formatCurrency } from "../utils"
import * as api from "../api"
import { useToast } from "./toast-context"

interface SellModalProps {
  portfolioId: string
  holding: Holding
  onConfirm: (soldHolding: Holding) => void
  onClose: () => void
}

export default function SellModal({ portfolioId, holding, onConfirm, onClose }: SellModalProps) {
  const [sellShares, setSellShares] = useState(
    holding.shares && holding.shares > 0 ? holding.shares.toString() : ""
  )
  const [sellPrice, setSellPrice] = useState(
    holding.shares && holding.shares > 0
      ? (holding.price || 0).toString()
      : holding.value.toString()
  )
  const [sellFee, setSellFee] = useState("")

  const { showToast } = useToast()

  const confirmSell = async () => {
    const feeNum = parseFloat(sellFee) || 0

    if (holding.shares && holding.shares > 0 && holding.price) {
      const sShares = parseFloat(sellShares)
      const sPrice = parseFloat(sellPrice)
      if (isNaN(sShares) || sShares <= 0) {
        showToast("请输入有效的卖出份额", "error")
        return
      }
      if (sShares > holding.shares) {
        showToast(`卖出份额不能超过持有量 ${holding.shares}`, "error")
        return
      }
      if (isNaN(sPrice) || sPrice < 0) {
        showToast("请输入有效的卖出价格", "error")
        return
      }

      try {
        const result = await api.sellHolding(portfolioId, holding.id, sShares, sPrice, feeNum, 0)
        onConfirm(result.soldHolding)
        onClose()
      } catch (e) {
        console.error("Failed to sell holding", e)
        showToast(e instanceof Error ? e.message : "卖出失败，请重试", "error")
      }
    } else {
      const sValue = parseFloat(sellPrice)
      if (isNaN(sValue) || sValue <= 0) {
        showToast("请输入有效的卖出金额", "error")
        return
      }
      if (sValue > holding.value) {
        showToast(`卖出金额不能超过持有值 ${formatCurrency(holding.value)}`, "error")
        return
      }

      try {
        const result = await api.sellHolding(portfolioId, holding.id, 0, 0, feeNum, sValue)
        onConfirm(result.soldHolding)
        onClose()
      } catch (e) {
        console.error("Failed to sell holding", e)
        showToast(e instanceof Error ? e.message : "卖出失败，请重试", "error")
      }
    }
  }

  return (
    <div className="fixed inset-0 bg-[#1A1A1A]/80 z-50 flex items-center justify-center p-4 backdrop-blur-sm">
      <div className="bg-white rounded-2xl max-w-sm w-full p-6 shadow-2xl flex flex-col gap-6">
        <div>
          <h3 className="text-lg font-bold text-[#1A1A1A]">卖出资产</h3>
          <p className="text-sm text-[#6C757D] mt-1">
            {holding.name || holding.symbol}
          </p>
        </div>

        <div className="space-y-4">
          {holding.shares && holding.shares > 0 ? (
            <>
              <div className="flex flex-col gap-2">
                <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                  卖出份额 (最多: {holding.shares})
                </label>
                <input
                  type="number"
                  value={sellShares}
                  onChange={(e) => setSellShares(e.target.value)}
                  className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
                />
              </div>
              <div className="flex flex-col gap-2">
                <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                  卖出单价 (参考: {holding.price || 0})
                </label>
                <input
                  type="number"
                  value={sellPrice}
                  onChange={(e) => setSellPrice(e.target.value)}
                  className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
                />
              </div>
            </>
          ) : (
            <div className="flex flex-col gap-2">
              <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                  卖出金额 (最多: {formatCurrency(holding.value)})
              </label>
              <input
                type="number"
                value={sellPrice}
                onChange={(e) => setSellPrice(e.target.value)}
                className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
                placeholder="输入要卖出的总价值"
              />
            </div>
          )}
        </div>

        <div className="flex flex-col gap-2">
          <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
            手续费 (选填)
          </label>
          <input
            type="number"
            value={sellFee}
            onChange={(e) => setSellFee(e.target.value)}
            className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
            placeholder="0"
          />
        </div>

        <div className="flex gap-3 justify-end pt-2 border-t border-[#F1F3F5]">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm font-medium text-[#6C757D] hover:bg-[#F8F9FA] rounded-xl transition-colors"
          >
            取消
          </button>
          <button
            onClick={confirmSell}
            className="px-4 py-2 text-sm font-medium text-white bg-orange-500 hover:bg-orange-600 rounded-xl transition-colors shadow-sm"
          >
            确认卖出
          </button>
        </div>
      </div>
    </div>
  )
}
