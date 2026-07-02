import { useState } from "react"
import { Holding } from "../types"
import * as api from "../api"
import { useToast } from "./toast-context"

interface BuyModalProps {
  portfolioId: string
  holding: Holding
  onConfirm: (updatedHolding: Holding) => void
  onClose: () => void
}

export default function BuyModal({ portfolioId, holding, onConfirm, onClose }: BuyModalProps) {
  const isSymbolBased = !!holding.symbol

  const [buyShares, setBuyShares] = useState("")
  const [buyPrice, setBuyPrice] = useState(holding.price ? holding.price.toString() : "")
  const [costCurrency, setCostCurrency] = useState(holding.currency || "CNY")
  const [buyFee, setBuyFee] = useState("")
  const [buyDate, setBuyDate] = useState(new Date().toISOString().split("T")[0])

  const [manualInputMode, setManualInputMode] = useState<"cost" | "priceShares">("cost")
  const [manualCost, setManualCost] = useState("")
  const [manualPrice, setManualPrice] = useState("")
  const [manualShares, setManualShares] = useState("")
  const [manualValue, setManualValue] = useState("")

  const [submitting, setSubmitting] = useState(false)
  const { showToast } = useToast()

  const handleConfirm = async () => {
    const feeNum = parseFloat(buyFee) || 0
    const dateMs = new Date(buyDate).getTime()

    if (isSymbolBased) {
      const sharesNum = parseFloat(buyShares)
      const priceNum = parseFloat(buyPrice)
      if (isNaN(sharesNum) || sharesNum <= 0) {
        showToast("请输入有效的买入份额", "error")
        return
      }
      if (isNaN(priceNum) || priceNum <= 0) {
        showToast("请输入有效的买入单价", "error")
        return
      }

      let finalCostPrice = priceNum
      const targetCurrency = holding.currency || "CNY"
      if (costCurrency !== targetCurrency) {
        try {
          const fxData = await api.fetchExchangeRate(`${costCurrency}${targetCurrency}`)
          if (fxData && fxData.rate) {
            finalCostPrice = priceNum * fxData.rate
          }
        } catch {
          showToast("汇率获取失败，使用原始价格", "info")
        }
      }

      setSubmitting(true)
      try {
        const result = await api.createHolding(portfolioId, {
          assetId: holding.assetId,
          symbol: holding.symbol,
          name: holding.name,
          market: holding.market,
          currency: targetCurrency,
          shares: sharesNum,
          price: holding.price,
          costPrice: finalCostPrice,
          value: sharesNum * holding.price,
          cost: sharesNum * finalCostPrice,
          fee: feeNum,
          date: dateMs,
        })
        onConfirm(result)
        onClose()
      } catch (e) {
        showToast(e instanceof Error ? e.message : "买入失败", "error")
      } finally {
        setSubmitting(false)
      }
    } else {
      let addedCost: number
      let addedShares: number
      if (manualInputMode === "priceShares") {
        const p = parseFloat(manualPrice)
        const s = parseFloat(manualShares)
        if (isNaN(p) || p <= 0 || isNaN(s) || s <= 0) {
          showToast("请输入有效的单价和份额", "error")
          return
        }
        addedCost = p * s
        addedShares = s
      } else {
        const c = parseFloat(manualCost)
        if (isNaN(c) || c <= 0) {
          showToast("请输入有效的总成本", "error")
          return
        }
        addedCost = c
        addedShares = 0
      }

      const val = parseFloat(manualValue)
      if (isNaN(val) || val <= 0) {
        showToast("请输入有效的当前价值", "error")
        return
      }

      setSubmitting(true)
      try {
        const result = await api.createHolding(portfolioId, {
          assetId: holding.assetId,
          symbol: "",
          name: holding.name,
          market: holding.market,
          currency: holding.currency || "CNY",
          shares: addedShares,
          price: manualInputMode === "priceShares" ? parseFloat(manualPrice) : 0,
          value: val,
          cost: addedCost,
          fee: feeNum,
          date: dateMs,
        })
        onConfirm(result)
        onClose()
      } catch (e) {
        showToast(e instanceof Error ? e.message : "买入失败", "error")
      } finally {
        setSubmitting(false)
      }
    }
  }

  return (
    <div className="fixed inset-0 bg-[#1A1A1A]/80 z-50 flex items-center justify-center p-4 backdrop-blur-sm">
      <div className="bg-white rounded-2xl max-w-sm w-full p-6 shadow-2xl flex flex-col gap-6">
        <div>
          <h3 className="text-lg font-bold text-[#1A1A1A]">买入资产</h3>
          <p className="text-sm text-[#6C757D] mt-1">{holding.name || holding.symbol}</p>
        </div>

        <div className="space-y-4">
          {isSymbolBased ? (
            <>
              <div className="flex flex-col gap-2">
                <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                  买入份额
                </label>
                <input
                  type="number"
                  value={buyShares}
                  onChange={(e) => setBuyShares(e.target.value)}
                  placeholder="0"
                  className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
                />
              </div>
              <div className="flex flex-col gap-2">
                <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                  买入单价 {holding.price > 0 && `(当前: ${holding.price})`}
                </label>
                <div className="flex w-full">
                  <select
                    value={costCurrency}
                    onChange={(e) => setCostCurrency(e.target.value)}
                    className="px-2 py-2 border border-r-0 border-[#E9ECEF] rounded-l-lg text-xs bg-gray-50 focus:outline-none focus:border-[#1A1A1A] w-17.5"
                  >
                    <option value="CNY">CNY</option>
                    <option value="USD">USD</option>
                    <option value="HKD">HKD</option>
                  </select>
                  <input
                    type="number"
                    value={buyPrice}
                    onChange={(e) => setBuyPrice(e.target.value)}
                    placeholder="均价"
                    className="w-full px-3 py-2 border border-[#E9ECEF] rounded-r-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono min-w-0"
                  />
                </div>
              </div>
            </>
          ) : (
            <>
              <div className="flex flex-col gap-2">
                <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                  录入方式
                </label>
                <div className="flex gap-3">
                  <label className="flex items-center gap-1.5 text-xs text-[#495057]">
                    <input
                      type="radio"
                      checked={manualInputMode === "cost"}
                      onChange={() => setManualInputMode("cost")}
                      className="text-[#1A1A1A] focus:ring-[#1A1A1A]"
                    />
                    总成本
                  </label>
                  <label className="flex items-center gap-1.5 text-xs text-[#495057]">
                    <input
                      type="radio"
                      checked={manualInputMode === "priceShares"}
                      onChange={() => setManualInputMode("priceShares")}
                      className="text-[#1A1A1A] focus:ring-[#1A1A1A]"
                    />
                    单价+份额
                  </label>
                </div>
              </div>
              {manualInputMode === "cost" ? (
                <div className="flex flex-col gap-2">
                  <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                    总成本
                  </label>
                  <input
                    type="number"
                    value={manualCost}
                    onChange={(e) => setManualCost(e.target.value)}
                    placeholder="投入本金"
                    className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
                  />
                </div>
              ) : (
                <>
                  <div className="flex flex-col gap-2">
                    <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                      单价
                    </label>
                    <input
                      type="number"
                      value={manualPrice}
                      onChange={(e) => setManualPrice(e.target.value)}
                      placeholder="买入单价"
                      className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
                    />
                  </div>
                  <div className="flex flex-col gap-2">
                    <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                      份额
                    </label>
                    <input
                      type="number"
                      value={manualShares}
                      onChange={(e) => setManualShares(e.target.value)}
                      placeholder="数量"
                      className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
                    />
                  </div>
                </>
              )}
              <div className="flex flex-col gap-2">
                <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                  当前价值
                </label>
                <input
                  type="number"
                  value={manualValue}
                  onChange={(e) => setManualValue(e.target.value)}
                  placeholder="最新估值"
                  className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
                />
              </div>
            </>
          )}

          <div className="flex flex-col gap-2">
            <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
              日期
            </label>
            <input
              type="date"
              value={buyDate}
              onChange={(e) => setBuyDate(e.target.value)}
              className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
            />
          </div>

          <div className="flex flex-col gap-2">
            <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
              手续费 (选填)
            </label>
            <input
              type="number"
              value={buyFee}
              onChange={(e) => setBuyFee(e.target.value)}
              placeholder="0"
              className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
            />
          </div>

        </div>

        <div className="flex gap-3 justify-end pt-2 border-t border-[#F1F3F5]">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm font-medium text-[#6C757D] hover:bg-[#F8F9FA] rounded-xl transition-colors"
          >
            取消
          </button>
          <button
            onClick={handleConfirm}
            disabled={submitting}
            className="px-4 py-2 text-sm font-medium text-white bg-[#1A1A1A] hover:opacity-90 rounded-xl transition-opacity shadow-sm disabled:opacity-50"
          >
            {submitting ? "提交中..." : "确认买入"}
          </button>
        </div>
      </div>
    </div>
  )
}
