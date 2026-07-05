import { useState } from "react"
import Decimal from "decimal.js"
import { Dividend, Holding } from "../types"
import * as api from "../api"
import { useToast } from "./toast-context"

interface DividendModalProps {
  portfolioId: string
  holding: Holding
  dividend?: Dividend
  onConfirm: (updatedHolding: Holding) => void
  onClose: () => void
}

export default function DividendModal({
  portfolioId,
  holding,
  dividend,
  onConfirm,
  onClose,
}: DividendModalProps) {
  const [amount, setAmount] = useState(dividend?.amount || "")
  const [taxWithheld, setTaxWithheld] = useState(dividend?.taxWithheld || "")
  const [currency, setCurrency] = useState(dividend?.currency || holding.currency || "CNY")
  const [dividendPerShare, setDividendPerShare] = useState(dividend?.dividendPerShare || "")
  const [payDate, setPayDate] = useState(
    dividend?.payDate
      ? new Date(dividend.payDate).toISOString().split("T")[0]
      : new Date().toISOString().split("T")[0]
  )
  const [reinvest, setReinvest] = useState(dividend?.reinvest || false)
  const [reinvestPrice, setReinvestPrice] = useState(
    dividend?.reinvestPrice || (holding.price ? holding.price.toString() : "")
  )
  const [note, setNote] = useState(dividend?.note || "")

  const [submitting, setSubmitting] = useState(false)
  const { showToast } = useToast()

  const netAmount = new Decimal(amount || "0").minus(new Decimal(taxWithheld || "0"))

  const handleConfirm = async () => {
    const amountNum = new Decimal(amount)
    if (amountNum.isNegative() || amountNum.isZero()) {
      showToast("请输入有效的分红金额", "error")
      return
    }

    const taxNum = new Decimal(taxWithheld || "0")
    if (taxNum.isNegative() || taxNum.greaterThan(amountNum)) {
      showToast("预扣税不能超过分红金额", "error")
      return
    }

    if (reinvest) {
      const priceNum = new Decimal(reinvestPrice)
      if (priceNum.isNegative() || priceNum.isZero()) {
        showToast("请输入有效的再投资价格", "error")
        return
      }
    }

    setSubmitting(true)
    try {
      if (dividend) {
        await api.updateDividend(portfolioId, dividend.id, {
          amount: amountNum.toString(),
          taxWithheld: taxNum.isZero() ? undefined : taxNum.toString(),
          currency,
          dividendPerShare: dividendPerShare || undefined,
          payDate: new Date(payDate).getTime() || undefined,
          reinvest,
          reinvestPrice: reinvest ? reinvestPrice : undefined,
          note: note || undefined,
        })
        onConfirm(holding)
        onClose()
        showToast("分红记录已更新", "success")
      } else {
        const result = await api.recordDividend(portfolioId, {
          holdingId: holding.id,
          amount: amountNum.toString(),
          taxWithheld: taxNum.isZero() ? undefined : taxNum.toString(),
          currency,
          dividendPerShare: dividendPerShare || undefined,
          payDate: new Date(payDate).getTime() || undefined,
          reinvest,
          reinvestPrice: reinvest ? reinvestPrice : undefined,
          note: note || undefined,
        })

        const updatedHolding: Holding = {
          ...holding,
          totalDividends: new Decimal(holding.totalDividends || "0")
            .plus(new Decimal(result.netAmount))
            .toString(),
        }

        if (result.reinvest && result.reinvestShares) {
          updatedHolding.shares = new Decimal(holding.shares || "0")
            .plus(new Decimal(result.reinvestShares))
            .toString()
          updatedHolding.cost = new Decimal(holding.cost || "0")
            .plus(new Decimal(result.netAmount))
            .toString()
        }

        onConfirm(updatedHolding)
        onClose()
        showToast("分红记录成功", "success")
      }
    } catch (e) {
      showToast(
        e instanceof Error ? e.message : dividend ? "更新分红失败" : "记录分红失败",
        "error"
      )
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-[#1A1A1A]/80 z-50 flex items-center justify-center p-4 backdrop-blur-sm">
      <div className="bg-white rounded-2xl max-w-sm w-full p-6 shadow-2xl flex flex-col gap-6">
        <div>
          <h3 className="text-lg font-bold text-[#1A1A1A]">{dividend ? "编辑分红" : "记录分红"}</h3>
          <p className="text-sm text-[#6C757D] mt-1">{holding.name || holding.symbol}</p>
        </div>

        <div className="space-y-4">
          <div className="flex flex-col gap-2">
            <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
              分红金额
            </label>
            <div className="flex w-full">
              <select
                value={currency}
                onChange={(e) => setCurrency(e.target.value)}
                className="px-2 py-2 border border-r-0 border-[#E9ECEF] rounded-l-lg text-xs bg-gray-50 focus:outline-none focus:border-[#1A1A1A] w-17.5"
              >
                <option value="CNY">CNY</option>
                <option value="USD">USD</option>
                <option value="HKD">HKD</option>
              </select>
              <input
                type="number"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                placeholder="0"
                className="w-full px-3 py-2 border border-[#E9ECEF] rounded-r-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono min-w-0"
              />
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
              预扣税 (选填)
            </label>
            <input
              type="number"
              value={taxWithheld}
              onChange={(e) => setTaxWithheld(e.target.value)}
              placeholder="0"
              className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
            />
          </div>

          <div className="flex flex-col gap-2">
            <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
              每股分红 (选填)
            </label>
            <input
              type="number"
              value={dividendPerShare}
              onChange={(e) => setDividendPerShare(e.target.value)}
              placeholder="0"
              className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
            />
          </div>

          <div className="flex flex-col gap-2">
            <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
              支付日期
            </label>
            <input
              type="date"
              value={payDate}
              onChange={(e) => setPayDate(e.target.value)}
              className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
            />
          </div>

          <div className="flex items-center gap-3">
            <input
              type="checkbox"
              id="reinvest"
              checked={reinvest}
              onChange={(e) => setReinvest(e.target.checked)}
              className="w-4 h-4 rounded border-[#DEE2E6] text-[#1A1A1A] focus:ring-[#1A1A1A]"
            />
            <label htmlFor="reinvest" className="text-sm text-[#495057]">
              红利再投资 (DRIP)
            </label>
          </div>

          {reinvest && (
            <div className="flex flex-col gap-2">
              <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                再投资价格
              </label>
              <input
                type="number"
                value={reinvestPrice}
                onChange={(e) => setReinvestPrice(e.target.value)}
                placeholder="当前市价"
                className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
              />
            </div>
          )}

          <div className="flex flex-col gap-2">
            <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
              备注 (选填)
            </label>
            <input
              type="text"
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder="例如：2023年第四季度分红"
              className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
            />
          </div>

          {netAmount.isPositive() && (
            <div className="bg-[#F8F9FA] rounded-lg p-3 text-sm">
              <span className="text-[#6C757D]">净分红金额: </span>
              <span className="font-medium text-[#1A1A1A]">
                {netAmount.toString()} {currency}
              </span>
            </div>
          )}
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
            {submitting ? "提交中..." : dividend ? "确认修改" : "确认记录"}
          </button>
        </div>
      </div>
    </div>
  )
}
