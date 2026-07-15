import { FormEvent, useCallback, useEffect, useId, useMemo, useState } from "react"
import {
  ArrowDownLeft,
  ArrowLeftRight,
  ArrowUpRight,
  Check,
  Info,
  LoaderCircle,
  MoveRight,
  RefreshCw,
  Repeat2,
  X,
} from "lucide-react"
import { AvailableFund, Portfolio } from "../types"
import * as api from "../api"
import { formatCurrencyByCode, toDecimal } from "../utils"
import { useToast } from "./toast-context"

type OperationType = "transfer_in" | "transfer_out" | "transfer" | "convert"

interface FundOperationDialogProps {
  type: OperationType
  portfolios: Portfolio[]
  availableFunds: AvailableFund[]
  currentPortfolioId: string
  currentCurrency?: string
  onClose: () => void
  onSuccess: () => void
}

const OPERATION_META = {
  transfer_in: {
    title: "转入资金",
    description: "记录一笔外部资金转入，增加当前组合的可用资金",
    icon: ArrowDownLeft,
    iconClass: "bg-emerald-50 text-emerald-700",
  },
  transfer_out: {
    title: "转出资金",
    description: "从当前组合提取资金，可用资金将相应减少",
    icon: ArrowUpRight,
    iconClass: "bg-orange-50 text-orange-700",
  },
  transfer: {
    title: "划转资金",
    description: "在投资组合之间移动资金，不影响整体资产总额",
    icon: Repeat2,
    iconClass: "bg-blue-50 text-blue-700",
  },
  convert: {
    title: "货币转换",
    description: "按指定汇率将一种可用货币兑换为另一种货币",
    icon: ArrowLeftRight,
    iconClass: "bg-violet-50 text-violet-700",
  },
} satisfies Record<OperationType, object>

const CURRENCIES = ["CNY", "USD", "HKD", "EUR", "GBP", "JPY"]
const fieldClass =
  "h-11 w-full rounded-xl border border-[#DEE2E6] bg-white px-3 text-sm text-[#1A1A1A] outline-none transition placeholder:text-[#ADB5BD] hover:border-[#ADB5BD] focus:border-[#1A1A1A] focus:ring-2 focus:ring-[#1A1A1A]/5 disabled:cursor-not-allowed disabled:bg-[#F8F9FA]"
const labelClass = "text-xs font-medium text-[#495057]"

function parseDecimalInput(value: string) {
  try {
    const parsed = toDecimal(value)
    return parsed.isFinite() && !parsed.isNaN() ? parsed : null
  } catch {
    return null
  }
}

export default function FundOperationDialog({
  type,
  portfolios,
  availableFunds,
  currentPortfolioId,
  currentCurrency,
  onClose,
  onSuccess,
}: FundOperationDialogProps) {
  const titleId = useId()
  const otherPortfolios = useMemo(
    () => portfolios.filter((portfolio) => portfolio.id !== currentPortfolioId),
    [portfolios, currentPortfolioId]
  )
  const currentPortfolio = portfolios.find((portfolio) => portfolio.id === currentPortfolioId)
  const availableCurrencies = [...new Set(availableFunds.map((fund) => fund.currency))]
  const usesAvailableCurrencies = type !== "transfer_in"
  const currencyOptions = usesAvailableCurrencies ? availableCurrencies : CURRENCIES
  const initialCurrency =
    currentCurrency && currencyOptions.includes(currentCurrency)
      ? currentCurrency
      : currencyOptions[0] || ""

  const [currency, setCurrency] = useState(initialCurrency)
  const [amount, setAmount] = useState("")
  const [note, setNote] = useState("")
  const [targetPortfolioId, setTargetPortfolioId] = useState(
    type === "transfer" && otherPortfolios.length > 0 ? otherPortfolios[0].id : ""
  )
  const [toCurrency, setToCurrency] = useState(
    type === "convert" ? CURRENCIES.find((item) => item !== initialCurrency) || "" : ""
  )
  const [toAmount, setToAmount] = useState("")
  const [exchangeRate, setExchangeRate] = useState("")
  const [fetchingRate, setFetchingRate] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState("")

  const { showToast } = useToast()
  const meta = OPERATION_META[type]
  const OperationIcon = meta.icon
  const availableBalance = availableFunds.find((fund) => fund.currency === currency)?.amount
  const amountValue = parseDecimalInput(amount)
  const toAmountValue = parseDecimalInput(toAmount)
  const rateValue = parseDecimalInput(exchangeRate)
  const hasInsufficientBalance = Boolean(
    usesAvailableCurrencies &&
    amountValue?.isPositive() &&
    availableBalance &&
    amountValue.gt(availableBalance)
  )
  const selectedTarget = otherPortfolios.find((portfolio) => portfolio.id === targetPortfolioId)
  const hasNoSourceFunds = usesAvailableCurrencies && currencyOptions.length === 0
  const hasNoTargetPortfolio = type === "transfer" && otherPortfolios.length === 0

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !submitting) onClose()
    }
    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [onClose, submitting])

  const updateConvertedAmount = (nextAmount: string, nextRate: string) => {
    const parsedAmount = parseDecimalInput(nextAmount)
    const parsedRate = parseDecimalInput(nextRate)
    setToAmount(
      parsedAmount?.isPositive() && parsedRate?.isPositive()
        ? parsedAmount.times(parsedRate).toString()
        : ""
    )
  }

  const fetchRate = useCallback(async () => {
    if (type !== "convert" || !currency || !toCurrency || currency === toCurrency) return
    setFetchingRate(true)
    setFormError("")
    try {
      const res = await api.fetchExchangeRate(`${currency}${toCurrency}`)
      if (res?.rate) {
        setExchangeRate(res.rate)
        updateConvertedAmount(amount, res.rate)
      } else {
        setFormError("暂时无法获取该币种的汇率，请手动输入")
      }
    } catch {
      setFormError("汇率获取失败，请稍后重试或手动输入")
    } finally {
      setFetchingRate(false)
    }
  }, [type, currency, toCurrency, amount])

  const handleAmountChange = (value: string) => {
    setAmount(value)
    setFormError("")
    if (type === "convert") updateConvertedAmount(value, exchangeRate)
  }

  const handleRateChange = (value: string) => {
    setExchangeRate(value)
    setFormError("")
    updateConvertedAmount(amount, value)
  }

  const handleToAmountChange = (value: string) => {
    setToAmount(value)
    setFormError("")
    const parsedAmount = parseDecimalInput(amount)
    const parsedTargetAmount = parseDecimalInput(value)
    if (parsedAmount?.isPositive() && parsedTargetAmount?.isPositive()) {
      setExchangeRate(parsedTargetAmount.div(parsedAmount).toString())
    }
  }

  const handleSourceCurrencyChange = (value: string) => {
    setCurrency(value)
    setFormError("")
    if (value === toCurrency) {
      setToCurrency(CURRENCIES.find((item) => item !== value) || "")
    }
    setExchangeRate("")
    setToAmount("")
  }

  const handleTargetCurrencyChange = (value: string) => {
    setToCurrency(value)
    setFormError("")
    setExchangeRate("")
    setToAmount("")
  }

  const handleSwap = () => {
    if (!availableCurrencies.includes(toCurrency)) return
    setCurrency(toCurrency)
    setToCurrency(currency)
    setAmount(toAmount)
    setToAmount(amount)
    setFormError("")
    const rate = parseDecimalInput(exchangeRate)
    setExchangeRate(rate?.isPositive() ? toDecimal(1).div(rate).toString() : "")
  }

  const setAmountByRatio = (ratio: number) => {
    if (!availableBalance) return
    const nextAmount = toDecimal(availableBalance).times(ratio).toString()
    handleAmountChange(nextAmount)
  }

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault()
    setFormError("")

    if (!amountValue?.isPositive()) {
      setFormError("请输入大于 0 的金额")
      return
    }
    if (hasInsufficientBalance) {
      setFormError(`金额不能超过可用余额 ${formatCurrencyByCode(availableBalance!, currency)}`)
      return
    }
    if (type === "transfer" && !targetPortfolioId) {
      setFormError("请选择目标组合")
      return
    }
    if (type === "convert" && (!toAmountValue?.isPositive() || !rateValue?.isPositive())) {
      setFormError("请填写有效的目标金额和汇率")
      return
    }

    setSubmitting(true)
    try {
      switch (type) {
        case "transfer_in":
          await api.transferInFunds(currentPortfolioId, currency, amount, note.trim())
          break
        case "transfer_out":
          await api.transferOutFunds(currentPortfolioId, currency, amount, note.trim())
          break
        case "transfer":
          await api.transferBetweenFunds(
            currentPortfolioId,
            currency,
            amount,
            targetPortfolioId,
            note.trim()
          )
          break
        case "convert":
          await api.convertCurrency(
            currentPortfolioId,
            currency,
            toCurrency,
            amount,
            toAmount,
            exchangeRate
          )
          break
      }
      showToast(`${meta.title}成功`, "success")
      onSuccess()
      onClose()
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "操作失败，请稍后重试")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-[#1A1A1A]/45 p-0 backdrop-blur-[2px] sm:items-center sm:p-4"
      onMouseDown={(event) => event.target === event.currentTarget && !submitting && onClose()}
    >
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className="flex max-h-[92dvh] w-full max-w-lg flex-col overflow-hidden rounded-t-3xl bg-white shadow-2xl sm:max-h-[min(760px,92vh)] sm:rounded-2xl"
      >
        <header className="flex items-start justify-between border-b border-[#F1F3F5] px-5 py-5 sm:px-6">
          <div className="flex min-w-0 items-start gap-3.5">
            <div
              className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-xl ${meta.iconClass}`}
            >
              <OperationIcon className="h-5 w-5" />
            </div>
            <div className="min-w-0">
              <h2 id={titleId} className="text-base font-semibold text-[#1A1A1A]">
                {meta.title}
              </h2>
              <p className="mt-1 text-xs leading-5 text-[#6C757D]">{meta.description}</p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            disabled={submitting}
            aria-label={`关闭${meta.title}`}
            className="-mr-2 ml-3 rounded-lg p-2 text-[#ADB5BD] transition-colors hover:bg-[#F1F3F5] hover:text-[#1A1A1A] disabled:cursor-not-allowed disabled:opacity-40"
          >
            <X className="h-5 w-5" />
          </button>
        </header>

        <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
          <div className="scrollbar-thin flex-1 space-y-5 overflow-y-auto px-5 py-5 sm:px-6">
            {currentPortfolio && (
              <div className="flex items-center justify-between rounded-xl bg-[#F8F9FA] px-3.5 py-3">
                <span className="text-xs text-[#868E96]">当前组合</span>
                <span className="max-w-[65%] truncate text-sm font-medium text-[#343A40]">
                  {currentPortfolio.name}
                </span>
              </div>
            )}

            {hasNoSourceFunds && (
              <div className="flex gap-2.5 rounded-xl border border-amber-200 bg-amber-50 px-3.5 py-3 text-xs leading-5 text-amber-800">
                <Info className="mt-0.5 h-4 w-4 shrink-0" />
                当前组合暂无可用资金，请先转入资金后再进行此操作。
              </div>
            )}

            {type === "convert" ? (
              <div className="space-y-4">
                <div className="grid grid-cols-[1fr_auto_1fr] items-end gap-2">
                  <div className="space-y-2">
                    <label htmlFor={`${titleId}-source-currency`} className={labelClass}>
                      卖出币种
                    </label>
                    <select
                      id={`${titleId}-source-currency`}
                      value={currency}
                      onChange={(event) => handleSourceCurrencyChange(event.target.value)}
                      disabled={hasNoSourceFunds}
                      className={fieldClass}
                    >
                      {currencyOptions.length === 0 ? (
                        <option value="">暂无可用币种</option>
                      ) : (
                        currencyOptions
                          .filter((item) => item !== toCurrency)
                          .map((item) => (
                            <option key={item} value={item}>
                              {item}
                            </option>
                          ))
                      )}
                    </select>
                  </div>
                  <button
                    type="button"
                    onClick={handleSwap}
                    disabled={!availableCurrencies.includes(toCurrency)}
                    className="mb-1 flex h-9 w-9 items-center justify-center rounded-full border border-[#E9ECEF] bg-white text-[#6C757D] transition hover:border-[#ADB5BD] hover:bg-[#F8F9FA] hover:text-[#1A1A1A] disabled:cursor-not-allowed disabled:opacity-35"
                    title={
                      availableCurrencies.includes(toCurrency)
                        ? "互换币种"
                        : "目标币种没有可用资金，无法互换"
                    }
                    aria-label="互换源币种与目标币种"
                  >
                    <ArrowLeftRight className="h-4 w-4" />
                  </button>
                  <div className="space-y-2">
                    <label htmlFor={`${titleId}-target-currency`} className={labelClass}>
                      买入币种
                    </label>
                    <select
                      id={`${titleId}-target-currency`}
                      value={toCurrency}
                      onChange={(event) => handleTargetCurrencyChange(event.target.value)}
                      className={fieldClass}
                    >
                      {CURRENCIES.filter((item) => item !== currency).map((item) => (
                        <option key={item} value={item}>
                          {item}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>

                <div className="rounded-2xl border border-[#E9ECEF] bg-[#FCFCFD] p-4">
                  <AmountField
                    id={`${titleId}-amount`}
                    label="卖出金额"
                    currency={currency}
                    value={amount}
                    onChange={handleAmountChange}
                    invalid={hasInsufficientBalance}
                    disabled={hasNoSourceFunds}
                  />
                  <BalanceAndRatios
                    currency={currency}
                    balance={availableBalance}
                    onRatio={setAmountByRatio}
                  />

                  <div className="my-4 flex items-center gap-3">
                    <div className="h-px flex-1 bg-[#E9ECEF]" />
                    <MoveRight className="h-4 w-4 text-[#ADB5BD]" />
                    <div className="h-px flex-1 bg-[#E9ECEF]" />
                  </div>

                  <AmountField
                    id={`${titleId}-target-amount`}
                    label="预计获得"
                    currency={toCurrency}
                    value={toAmount}
                    onChange={handleToAmountChange}
                  />
                </div>

                <div className="space-y-2">
                  <div className="flex items-center justify-between gap-3">
                    <label htmlFor={`${titleId}-rate`} className={labelClass}>
                      兑换汇率
                    </label>
                    <button
                      type="button"
                      onClick={fetchRate}
                      disabled={fetchingRate || hasNoSourceFunds}
                      className="inline-flex items-center gap-1.5 text-xs font-medium text-blue-600 transition hover:text-blue-800 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      <RefreshCw className={`h-3.5 w-3.5 ${fetchingRate ? "animate-spin" : ""}`} />
                      {fetchingRate ? "正在获取" : "获取最新汇率"}
                    </button>
                  </div>
                  <div className="relative">
                    <input
                      id={`${titleId}-rate`}
                      type="number"
                      inputMode="decimal"
                      min="0"
                      step="any"
                      value={exchangeRate}
                      onChange={(event) => handleRateChange(event.target.value)}
                      placeholder="0.0000"
                      className={`${fieldClass} pr-32 font-mono`}
                    />
                    <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-[#868E96]">
                      1 {currency} = ? {toCurrency}
                    </span>
                  </div>
                </div>
              </div>
            ) : (
              <div className="space-y-5">
                <div className="space-y-2">
                  <label htmlFor={`${titleId}-currency`} className={labelClass}>
                    币种
                  </label>
                  <select
                    id={`${titleId}-currency`}
                    value={currency}
                    onChange={(event) => handleSourceCurrencyChange(event.target.value)}
                    disabled={hasNoSourceFunds}
                    className={fieldClass}
                  >
                    {currencyOptions.length === 0 ? (
                      <option value="">暂无可用币种</option>
                    ) : (
                      currencyOptions.map((item) => (
                        <option key={item} value={item}>
                          {item}
                        </option>
                      ))
                    )}
                  </select>
                </div>

                <div className="space-y-2">
                  <AmountField
                    id={`${titleId}-amount`}
                    label={type === "transfer_in" ? "转入金额" : "操作金额"}
                    currency={currency}
                    value={amount}
                    onChange={handleAmountChange}
                    invalid={hasInsufficientBalance}
                    disabled={hasNoSourceFunds}
                  />
                  {usesAvailableCurrencies && (
                    <BalanceAndRatios
                      currency={currency}
                      balance={availableBalance}
                      onRatio={setAmountByRatio}
                    />
                  )}
                </div>

                {type === "transfer" && (
                  <div className="space-y-2">
                    <label htmlFor={`${titleId}-target-portfolio`} className={labelClass}>
                      转入组合
                    </label>
                    <select
                      id={`${titleId}-target-portfolio`}
                      value={targetPortfolioId}
                      onChange={(event) => setTargetPortfolioId(event.target.value)}
                      disabled={hasNoTargetPortfolio}
                      className={fieldClass}
                    >
                      {hasNoTargetPortfolio ? (
                        <option value="">暂无其他投资组合</option>
                      ) : (
                        otherPortfolios.map((portfolio) => (
                          <option key={portfolio.id} value={portfolio.id}>
                            {portfolio.name}
                          </option>
                        ))
                      )}
                    </select>
                    {hasNoTargetPortfolio && (
                      <p className="text-xs text-amber-700">请先创建另一个投资组合再进行划转。</p>
                    )}
                  </div>
                )}

                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <label htmlFor={`${titleId}-note`} className={labelClass}>
                      备注
                    </label>
                    <span className="text-[11px] text-[#ADB5BD]">选填</span>
                  </div>
                  <input
                    id={`${titleId}-note`}
                    type="text"
                    value={note}
                    onChange={(event) => setNote(event.target.value)}
                    maxLength={100}
                    placeholder="例如：月度入金、生活备用金"
                    className={fieldClass}
                  />
                </div>
              </div>
            )}

            {formError && (
              <div
                role="alert"
                className="flex gap-2 rounded-xl bg-red-50 px-3.5 py-3 text-xs leading-5 text-red-700"
              >
                <Info className="mt-0.5 h-4 w-4 shrink-0" />
                {formError}
              </div>
            )}

            {amountValue?.isPositive() && !hasInsufficientBalance && (
              <div className="flex items-start gap-2.5 rounded-xl border border-[#E9ECEF] px-3.5 py-3">
                <Check className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" />
                <div className="min-w-0 text-xs leading-5 text-[#6C757D]">
                  {type === "convert" ? (
                    <>
                      将 {formatCurrencyByCode(amount, currency)} 兑换为约{" "}
                      <strong className="font-semibold text-[#343A40]">
                        {toAmountValue?.isPositive()
                          ? formatCurrencyByCode(toAmount, toCurrency)
                          : `— ${toCurrency}`}
                      </strong>
                    </>
                  ) : type === "transfer" ? (
                    <>
                      {formatCurrencyByCode(amount, currency)} 将划转至{" "}
                      <strong className="font-semibold text-[#343A40]">
                        {selectedTarget?.name || "目标组合"}
                      </strong>
                    </>
                  ) : (
                    <>
                      当前组合可用资金将
                      {type === "transfer_in" ? "增加" : "减少"}{" "}
                      <strong className="font-semibold text-[#343A40]">
                        {formatCurrencyByCode(amount, currency)}
                      </strong>
                    </>
                  )}
                </div>
              </div>
            )}
          </div>

          <footer className="flex gap-3 border-t border-[#E9ECEF] bg-white px-5 py-4 sm:justify-end sm:px-6">
            <button
              type="button"
              onClick={onClose}
              disabled={submitting}
              className="h-11 flex-1 rounded-xl border border-[#DEE2E6] px-5 text-sm font-medium text-[#495057] transition hover:bg-[#F8F9FA] disabled:cursor-not-allowed disabled:opacity-40 sm:flex-none"
            >
              取消
            </button>
            <button
              type="submit"
              disabled={submitting || hasNoSourceFunds || hasNoTargetPortfolio}
              className="inline-flex h-11 flex-[1.5] items-center justify-center gap-2 rounded-xl bg-[#1A1A1A] px-6 text-sm font-medium text-white transition hover:bg-[#343A40] disabled:cursor-not-allowed disabled:opacity-40 sm:flex-none"
            >
              {submitting && <LoaderCircle className="h-4 w-4 animate-spin" />}
              {submitting ? "处理中…" : "确认"}
            </button>
          </footer>
        </form>
      </section>
    </div>
  )
}

function AmountField({
  id,
  label,
  currency,
  value,
  onChange,
  invalid = false,
  disabled = false,
}: {
  id: string
  label: string
  currency: string
  value: string
  onChange: (value: string) => void
  invalid?: boolean
  disabled?: boolean
}) {
  return (
    <div className="space-y-2">
      <label htmlFor={id} className={labelClass}>
        {label}
      </label>
      <div className="relative">
        <input
          id={id}
          type="number"
          inputMode="decimal"
          min="0"
          step="any"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          disabled={disabled}
          aria-invalid={invalid}
          placeholder="0.00"
          className={`${fieldClass} pr-16 font-mono text-base ${invalid ? "border-red-400 focus:border-red-500 focus:ring-red-500/10" : ""}`}
        />
        <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs font-medium text-[#868E96]">
          {currency}
        </span>
      </div>
    </div>
  )
}

function BalanceAndRatios({
  currency,
  balance,
  onRatio,
}: {
  currency: string
  balance?: string
  onRatio: (ratio: number) => void
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-2">
      <p className="text-[11px] text-[#868E96]">
        可用余额{" "}
        <span className="font-mono font-medium text-[#495057]">
          {balance ? formatCurrencyByCode(balance, currency) : `— ${currency}`}
        </span>
      </p>
      {balance && (
        <div className="flex items-center gap-1">
          {[
            [0.25, "25%"],
            [0.5, "50%"],
            [1, "全部"],
          ].map(([ratio, label]) => (
            <button
              key={label}
              type="button"
              onClick={() => onRatio(Number(ratio))}
              className="rounded-md bg-[#F1F3F5] px-2 py-1 text-[10px] font-medium text-[#6C757D] transition hover:bg-[#E9ECEF] hover:text-[#1A1A1A]"
            >
              {label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
