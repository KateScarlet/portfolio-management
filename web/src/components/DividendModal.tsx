import { useMemo, useState } from "react"
import Decimal from "decimal.js"
import { Dividend, Holding } from "../types"
import * as api from "../api"
import { useToast } from "./toast-context"

interface DividendModalProps {
  portfolioId: string
  holding: Holding
  dividend?: Dividend
  onConfirm: (savedDividend: Dividend) => void | Promise<void>
  onClose: () => void
}

function localDateValue(value?: string): string {
  const date = value ? new Date(value) : new Date()
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 10)
}

function parseDecimal(value: string): Decimal | null {
  try {
    const parsed = new Decimal(value)
    return parsed.isFinite() ? parsed : null
  } catch {
    return null
  }
}

export default function DividendModal({
  portfolioId,
  holding,
  dividend,
  onConfirm,
  onClose,
}: DividendModalProps) {
  const [grossAmount, setGrossAmount] = useState(dividend?.grossAmount ?? "")
  const [taxAmount, setTaxAmount] = useState(dividend?.taxAmount ?? "0")
  const [type, setType] = useState<"cash" | "reinvest">(dividend?.type ?? "cash")
  const [paymentDate, setPaymentDate] = useState(localDateValue(dividend?.paymentDate))
  const [reinvestmentPrice, setReinvestmentPrice] = useState(
    dividend?.reinvestmentPrice && new Decimal(dividend.reinvestmentPrice).isPositive()
      ? dividend.reinvestmentPrice
      : holding.price || ""
  )
  const [note, setNote] = useState(dividend?.note ?? "")
  const [submitting, setSubmitting] = useState(false)
  const { showToast } = useToast()

  const netAmount = useMemo(() => {
    const gross = parseDecimal(grossAmount)
    const tax = parseDecimal(taxAmount || "0")
    return gross && tax ? gross.minus(tax) : null
  }, [grossAmount, taxAmount])

  const submit = async () => {
    const gross = parseDecimal(grossAmount)
    const tax = parseDecimal(taxAmount || "0")
    const price = parseDecimal(reinvestmentPrice)
    if (!gross?.isPositive()) {
      showToast("请输入大于 0 的分红总额", "error")
      return
    }
    if (!tax || tax.isNegative() || tax.greaterThanOrEqualTo(gross)) {
      showToast("预扣税必须大于等于 0 且小于分红总额", "error")
      return
    }
    if (!paymentDate) {
      showToast("请选择支付日期", "error")
      return
    }
    if (type === "reinvest" && !price?.isPositive()) {
      showToast("请输入大于 0 的再投资价格", "error")
      return
    }

    const payload = {
      grossAmount: gross.toString(),
      taxAmount: tax.toString(),
      type,
      paymentDate: new Date(`${paymentDate}T00:00:00`).toISOString(),
      reinvestmentPrice: type === "reinvest" ? price!.toString() : "0",
      note: note.trim() || undefined,
    }

    setSubmitting(true)
    try {
      let savedDividend: Dividend
      if (dividend) {
        savedDividend = await api.updateDividend(portfolioId, dividend.id, payload)
      } else {
        savedDividend = await api.recordDividend(portfolioId, {
          holdingId: holding.id,
          ...payload,
        })
      }
      await onConfirm(savedDividend)
      onClose()
      showToast(dividend ? "分红记录已更新" : "分红记录成功", "success")
    } catch (error) {
      showToast(error instanceof Error ? error.message : "保存分红失败", "error")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-[#1A1A1A]/80 z-50 flex items-center justify-center p-4 backdrop-blur-sm">
      <div className="bg-white rounded-2xl max-w-sm w-full p-6 shadow-2xl flex flex-col gap-6">
        <div>
          <h3 className="text-lg font-bold text-[#1A1A1A]">{dividend ? "编辑分红" : "记录分红"}</h3>
          <p className="text-sm text-[#6C757D] mt-1">
            {holding.name || holding.symbol} · {holding.currency || "CNY"}
          </p>
        </div>

        <div className="space-y-4">
          <label className="flex flex-col gap-2 text-xs font-bold text-[#6C757D]">
            分红总额
            <input
              type="number"
              min="0"
              step="any"
              value={grossAmount}
              onChange={(event) => setGrossAmount(event.target.value)}
              className="px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm font-mono font-normal"
            />
          </label>
          <label className="flex flex-col gap-2 text-xs font-bold text-[#6C757D]">
            预扣税
            <input
              type="number"
              min="0"
              step="any"
              value={taxAmount}
              onChange={(event) => setTaxAmount(event.target.value)}
              className="px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm font-mono font-normal"
            />
          </label>
          <label className="flex flex-col gap-2 text-xs font-bold text-[#6C757D]">
            支付日期
            <input
              type="date"
              value={paymentDate}
              onChange={(event) => setPaymentDate(event.target.value)}
              className="px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm font-normal"
            />
          </label>
          <div className="grid grid-cols-2 gap-2">
            <button
              type="button"
              onClick={() => setType("cash")}
              className={`px-3 py-2 rounded-lg text-sm border ${type === "cash" ? "bg-[#1A1A1A] text-white border-[#1A1A1A]" : "border-[#DEE2E6] text-[#6C757D]"}`}
            >
              现金分红
            </button>
            <button
              type="button"
              onClick={() => setType("reinvest")}
              className={`px-3 py-2 rounded-lg text-sm border ${type === "reinvest" ? "bg-[#1A1A1A] text-white border-[#1A1A1A]" : "border-[#DEE2E6] text-[#6C757D]"}`}
            >
              红利再投资
            </button>
          </div>
          {type === "reinvest" && (
            <label className="flex flex-col gap-2 text-xs font-bold text-[#6C757D]">
              再投资价格
              <input
                type="number"
                min="0"
                step="any"
                value={reinvestmentPrice}
                onChange={(event) => setReinvestmentPrice(event.target.value)}
                className="px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm font-mono font-normal"
              />
            </label>
          )}
          <label className="flex flex-col gap-2 text-xs font-bold text-[#6C757D]">
            备注
            <input
              type="text"
              maxLength={500}
              value={note}
              onChange={(event) => setNote(event.target.value)}
              className="px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm font-normal"
            />
          </label>
          {netAmount?.isPositive() && (
            <div className="bg-[#F8F9FA] rounded-lg p-3 text-sm">
              <span className="text-[#6C757D]">净分红：</span>
              <span className="font-medium">
                {netAmount.toString()} {holding.currency || "CNY"}
              </span>
              {type === "reinvest" && parseDecimal(reinvestmentPrice)?.isPositive() && (
                <div className="text-xs text-[#6C757D] mt-1">
                  预计新增 {netAmount.div(reinvestmentPrice).toString()} 份
                </div>
              )}
            </div>
          )}
        </div>

        <div className="flex gap-3 justify-end pt-2 border-t border-[#F1F3F5]">
          <button type="button" onClick={onClose} className="px-4 py-2 text-sm text-[#6C757D]">
            取消
          </button>
          <button
            type="button"
            onClick={submit}
            disabled={submitting}
            className="px-4 py-2 text-sm text-white bg-[#1A1A1A] rounded-xl disabled:opacity-50"
          >
            {submitting ? "提交中..." : "确认"}
          </button>
        </div>
      </div>
    </div>
  )
}
