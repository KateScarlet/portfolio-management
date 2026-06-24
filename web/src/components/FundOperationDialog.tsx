import { useState, useCallback } from "react"
import { Portfolio } from "../types"
import * as api from "../api"
import { useToast } from "./toast-context"

type OperationType = "transfer_in" | "transfer_out" | "transfer" | "convert"

interface FundOperationDialogProps {
  type: OperationType
  portfolios: Portfolio[]
  currentPortfolioId: string
  currentCurrency?: string
  onClose: () => void
  onSuccess: () => void
}

const OPERATION_TITLES: Record<OperationType, string> = {
  transfer_in: "转入资金",
  transfer_out: "转出资金",
  transfer: "划转资金",
  convert: "货币转换",
}

const CURRENCIES = ["CNY", "USD", "HKD", "EUR", "GBP", "JPY"]

export default function FundOperationDialog({
  type,
  portfolios,
  currentPortfolioId,
  currentCurrency,
  onClose,
  onSuccess,
}: FundOperationDialogProps) {
  const otherPortfolios = portfolios.filter((p) => p.id !== currentPortfolioId)

  const [currency, setCurrency] = useState(currentCurrency || "CNY")
  const [amount, setAmount] = useState("")
  const [note, setNote] = useState("")
  const [targetPortfolioId, setTargetPortfolioId] = useState(
    type === "transfer" && otherPortfolios.length > 0 ? otherPortfolios[0].id : ""
  )
  const [toCurrency, setToCurrency] = useState(
    type === "convert" ? CURRENCIES.find((c) => c !== (currentCurrency || "CNY")) || "USD" : ""
  )
  const [toAmount, setToAmount] = useState("")
  const [exchangeRate, setExchangeRate] = useState("")
  const [fetchingRate, setFetchingRate] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const { showToast } = useToast()

  const fetchRate = useCallback(async () => {
    if (type !== "convert" || !currency || !toCurrency || currency === toCurrency) return
    setFetchingRate(true)
    try {
      const res = await api.fetchExchangeRate(`${currency}${toCurrency}`)
      if (res && res.rate) {
        setExchangeRate(res.rate.toFixed(4))
        const amt = parseFloat(amount)
        if (!isNaN(amt) && amt > 0) {
          setToAmount((amt * res.rate).toFixed(2))
        }
      }
    } catch {
      showToast("汇率获取失败", "error")
    } finally {
      setFetchingRate(false)
    }
  }, [type, currency, toCurrency, amount, showToast])

  const handleAmountChange = (val: string) => {
    setAmount(val)
    if (type === "convert" && exchangeRate) {
      const amt = parseFloat(val)
      const rate = parseFloat(exchangeRate)
      if (!isNaN(amt) && !isNaN(rate) && amt > 0 && rate > 0) {
        setToAmount((amt * rate).toFixed(2))
      }
    }
  }

  const handleRateChange = (val: string) => {
    setExchangeRate(val)
    const amt = parseFloat(amount)
    const rate = parseFloat(val)
    if (!isNaN(amt) && !isNaN(rate) && amt > 0 && rate > 0) {
      setToAmount((amt * rate).toFixed(2))
    }
  }

  const handleToAmountChange = (val: string) => {
    setToAmount(val)
    const amt = parseFloat(amount)
    const toAmt = parseFloat(val)
    if (!isNaN(amt) && !isNaN(toAmt) && amt > 0 && toAmt > 0) {
      setExchangeRate((toAmt / amt).toFixed(4))
    }
  }

  const handleSubmit = async () => {
    const amt = parseFloat(amount)
    if (isNaN(amt) || amt <= 0) {
      showToast("请输入有效金额", "error")
      return
    }

    setSubmitting(true)
    try {
      switch (type) {
        case "transfer_in":
          await api.transferInFunds(currentPortfolioId, currency, amt, note)
          break
        case "transfer_out":
          await api.transferOutFunds(currentPortfolioId, currency, amt, note)
          break
        case "transfer":
          if (!targetPortfolioId) {
            showToast("请选择目标组合", "error")
            setSubmitting(false)
            return
          }
          await api.transferBetweenFunds(currentPortfolioId, currency, amt, targetPortfolioId, note)
          break
        case "convert": {
          const toAmt = parseFloat(toAmount)
          const rate = parseFloat(exchangeRate)
          if (isNaN(toAmt) || toAmt <= 0) {
            showToast("请输入有效的目标金额", "error")
            setSubmitting(false)
            return
          }
          if (isNaN(rate) || rate <= 0) {
            showToast("请输入有效的汇率", "error")
            setSubmitting(false)
            return
          }
          await api.convertCurrency(currentPortfolioId, currency, toCurrency, amt, toAmt, rate)
          break
        }
      }
      showToast("操作成功", "success")
      onSuccess()
      onClose()
    } catch (e) {
      showToast(e instanceof Error ? e.message : "操作失败", "error")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/30"
      onClick={onClose}
    >
      <div
        className="bg-white rounded-xl shadow-lg border border-[#E9ECEF] w-full max-w-md mx-4"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-5 py-4 border-b border-[#E9ECEF]">
          <h3 className="text-sm font-semibold text-[#1A1A1A]">{OPERATION_TITLES[type]}</h3>
          <button
            onClick={onClose}
            className="text-[#ADB5BD] hover:text-[#1A1A1A] text-lg leading-none"
          >
            &times;
          </button>
        </div>

        <div className="p-5 flex flex-col gap-4">
          {type === "convert" ? (
            <>
              <div className="grid grid-cols-2 gap-3">
                <div className="flex flex-col gap-1">
                  <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                    源币种
                  </label>
                  <select
                    value={currency}
                    onChange={(e) => setCurrency(e.target.value)}
                    className="px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
                  >
                    {CURRENCIES.filter((c) => c !== toCurrency).map((c) => (
                      <option key={c} value={c}>
                        {c}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="flex flex-col gap-1">
                  <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                    目标币种
                  </label>
                  <select
                    value={toCurrency}
                    onChange={(e) => setToCurrency(e.target.value)}
                    className="px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
                  >
                    {CURRENCIES.filter((c) => c !== currency).map((c) => (
                      <option key={c} value={c}>
                        {c}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="flex flex-col gap-1">
                  <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                    源金额
                  </label>
                  <input
                    type="number"
                    value={amount}
                    onChange={(e) => handleAmountChange(e.target.value)}
                    placeholder="0.00"
                    className="px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm font-mono focus:outline-none focus:border-[#1A1A1A]"
                  />
                </div>
                <div className="flex flex-col gap-1">
                  <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                    目标金额
                  </label>
                  <input
                    type="number"
                    value={toAmount}
                    onChange={(e) => handleToAmountChange(e.target.value)}
                    placeholder="0.00"
                    className="px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm font-mono focus:outline-none focus:border-[#1A1A1A]"
                  />
                </div>
              </div>
              <div className="flex flex-col gap-1">
                <div className="flex items-center justify-between">
                  <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                    汇率
                  </label>
                  <button
                    onClick={fetchRate}
                    disabled={fetchingRate}
                    className="text-[10px] text-blue-600 hover:text-blue-800 disabled:opacity-50"
                  >
                    {fetchingRate ? "获取中..." : "刷新汇率"}
                  </button>
                </div>
                <input
                  type="number"
                  value={exchangeRate}
                  onChange={(e) => handleRateChange(e.target.value)}
                  placeholder="汇率"
                  step="0.0001"
                  className="px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm font-mono focus:outline-none focus:border-[#1A1A1A]"
                />
              </div>
            </>
          ) : (
            <>
              <div className="flex flex-col gap-1">
                <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                  币种
                </label>
                <select
                  value={currency}
                  onChange={(e) => setCurrency(e.target.value)}
                  className="px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
                >
                  {CURRENCIES.map((c) => (
                    <option key={c} value={c}>
                      {c}
                    </option>
                  ))}
                </select>
              </div>
              <div className="flex flex-col gap-1">
                <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                  金额
                </label>
                <input
                  type="number"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                  placeholder="0.00"
                  className="px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm font-mono focus:outline-none focus:border-[#1A1A1A]"
                />
              </div>
              {type === "transfer" && (
                <div className="flex flex-col gap-1">
                  <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                    目标组合
                  </label>
                  <select
                    value={targetPortfolioId}
                    onChange={(e) => setTargetPortfolioId(e.target.value)}
                    className="px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
                  >
                    {otherPortfolios.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name}
                      </option>
                    ))}
                  </select>
                </div>
              )}
            </>
          )}

          <div className="flex flex-col gap-1">
            <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
              备注 (选填)
            </label>
            <input
              type="text"
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder="备注信息"
              className="px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm focus:outline-none focus:border-[#1A1A1A]"
            />
          </div>
        </div>

        <div className="flex justify-end gap-2 px-5 py-4 border-t border-[#E9ECEF]">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm text-[#6C757D] hover:text-[#1A1A1A] transition-colors"
          >
            取消
          </button>
          <button
            onClick={handleSubmit}
            disabled={submitting}
            className="px-4 py-2 text-sm text-white bg-[#1A1A1A] rounded-lg hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {submitting ? "提交中..." : "确认"}
          </button>
        </div>
      </div>
    </div>
  )
}
