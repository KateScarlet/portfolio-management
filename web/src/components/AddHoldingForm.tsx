import { useState } from "react"
import { AssetId, ASSET_DEFINITIONS, COMMODITY_SYMBOLS, CRYPTO_SYMBOLS } from "../types"
import * as api from "../api"
import { useToast } from "./toast-context"

interface AddHoldingFormProps {
  onAddHolding: (holding: Parameters<typeof api.createHolding>[0]) => Promise<void>
  onClose: () => void
}

export default function AddHoldingForm({
  onAddHolding,
  onClose,
}: AddHoldingFormProps) {
  const [assetId, setAssetId] = useState<AssetId>("stocks")
  const [market, setMarket] = useState<"US" | "CN" | "HK" | "COMMODITY" | "CRYPTO">("US")
  const [symbol, setSymbol] = useState("")
  const [name, setName] = useState("")
  const [shares, setShares] = useState("")
  const [costPrice, setCostPrice] = useState("")
  const [costCurrency, setCostCurrency] = useState("CNY")
  const [value, setValue] = useState("")
  const [cost, setCost] = useState("")
  const [fee, setFee] = useState("")
  const [date, setDate] = useState(new Date().toISOString().split("T")[0])
  const [isManual, setIsManual] = useState(false)
  const [manualInputMode, setManualInputMode] = useState<"cost" | "priceShares">("cost")
  const [manualPrice, setManualPrice] = useState("")
  const [manualShares, setManualShares] = useState("")
  const [isFetching, setIsFetching] = useState(false)
  const [deductFromCash, setDeductFromCash] = useState(true)

  const { showToast } = useToast()

  const handleAdd = async () => {
    const feeNum = parseFloat(fee) || 0

    if (isManual) {
      const val = parseFloat(value)
      if (isNaN(val) || val <= 0) {
        showToast("请输入有效的当前总市值", "error")
        return
      }

      let addedCost: number
      if (manualInputMode === "priceShares") {
        const p = parseFloat(manualPrice)
        const s = parseFloat(manualShares)
        if (isNaN(p) || p <= 0 || isNaN(s) || s <= 0) {
          showToast("请输入有效的单价和份额", "error")
          return
        }
        addedCost = p * s
      } else {
        const c = parseFloat(cost)
        addedCost = (isNaN(c) || c <= 0 ? val : c)
      }
      try {
        await onAddHolding({
          assetId,
          symbol: "",
          name: name.trim() || "手工资产",
          shares: manualInputMode === "priceShares" ? (parseFloat(manualShares) || 0) : 0,
          price: manualInputMode === "priceShares" ? (parseFloat(manualPrice) || 0) : 0,
          value: val,
          cost: addedCost,
          fee: feeNum,
          date: new Date(date).getTime(),
          deductFromCash: deductFromCash,
        })
      } catch (e) {
        showToast(e instanceof Error ? e.message : "录入失败", "error")
        return
      }
    } else {
      const sharesNum = parseFloat(shares)
      const cPrice = parseFloat(costPrice)
      if (!symbol) {
        showToast("请输入股票/基金代码", "error")
        return
      }
      if (isNaN(sharesNum) || sharesNum <= 0) {
        showToast("请输入有效的份额", "error")
        return
      }

      setIsFetching(true)
      try {
        const data = await api.fetchPrice(symbol)
        if (data && data.price) {
          const authoritativeSymbol = data.symbol || symbol.toUpperCase()
          let finalCostPrice = isNaN(cPrice) || cPrice <= 0 ? data.price : cPrice

          if (!isNaN(cPrice) && cPrice > 0 && costCurrency !== "CNY") {
            try {
              const fxData = await api.fetchExchangeRate(`${costCurrency}CNY`)
              if (fxData && fxData.rate) {
                finalCostPrice = cPrice * fxData.rate
              }
            } catch {
              showToast("汇率获取失败，使用原始价格", "info")
            }
          }

          await onAddHolding({
            assetId,
            symbol: authoritativeSymbol,
            name: name.trim() || data.name || authoritativeSymbol,
            shares: sharesNum,
            price: data.price,
            costPrice: finalCostPrice,
            value: sharesNum * data.price,
            cost: sharesNum * finalCostPrice,
            fee: feeNum,
            date: new Date(date).getTime(),
            deductFromCash: deductFromCash,
          })
        } else {
          showToast("价格获取失败，请尝试手动录入", "error")
          setIsFetching(false)
          return
        }
      } catch (e) {
        if (e instanceof Error && e.message.includes("可用资金")) {
          showToast(e.message, "error")
        } else {
          showToast("价格获取失败，请尝试手动录入", "error")
        }
        setIsFetching(false)
        return
      } finally {
        setIsFetching(false)
      }
    }

    onClose()
  }

  return (
    <div className="p-6 bg-[#F8F9FA] border-b border-[#E9ECEF] flex flex-col gap-4">
      <h4 className="text-sm font-medium text-[#1A1A1A]">添加新资产</h4>
      <div className="flex gap-4 mb-2">
        <label className="flex items-center gap-2 text-sm text-[#495057]">
          <input
            type="radio"
            checked={!isManual}
            onChange={() => setIsManual(false)}
            className="text-[#1A1A1A] focus:ring-[#1A1A1A]"
          />
          自动获取价格
        </label>
        <label className="flex items-center gap-2 text-sm text-[#495057]">
          <input
            type="radio"
            checked={isManual}
            onChange={() => setIsManual(true)}
            className="text-[#1A1A1A] focus:ring-[#1A1A1A]"
          />
          手动录入
        </label>
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 xl:grid-cols-8 gap-4">
        <div className="flex flex-col gap-1">
          <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
            分类
          </label>
          <select
            value={assetId}
            onChange={(e) => setAssetId(e.target.value as AssetId)}
            className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
          >
            {Object.keys(ASSET_DEFINITIONS).map((key) => (
              <option key={key} value={key}>
                {ASSET_DEFINITIONS[key as AssetId].name}
              </option>
            ))}
          </select>
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
            购买/录入日期
          </label>
          <input
            type="date"
            value={date}
            onChange={(e) => setDate(e.target.value)}
            className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
          />
        </div>

        {!isManual ? (
          <>
            <div className="flex flex-col gap-1">
              <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                市场
              </label>
              <select
                value={market}
                onChange={(e) => {
                  setMarket(e.target.value as "US" | "CN" | "HK" | "COMMODITY" | "CRYPTO")
                  setSymbol("")
                }}
                className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
              >
                <option value="US">美股</option>
                <option value="CN">A股</option>
                <option value="HK">港股</option>
                <option value="COMMODITY">期货</option>
                <option value="CRYPTO">加密货币</option>
              </select>
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                {market === "COMMODITY" ? "商品" : market === "CRYPTO" ? "币种" : "代码"}
              </label>
              {market === "COMMODITY" ? (
                <select
                  value={symbol}
                  onChange={(e) => setSymbol(e.target.value)}
                  className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
                >
                  <option value="">选择商品...</option>
                  {COMMODITY_SYMBOLS.map((c) => (
                    <option key={c.symbol} value={c.symbol}>
                      {c.name} ({c.symbol})
                    </option>
                  ))}
                </select>
              ) : market === "CRYPTO" ? (
                <select
                  value={symbol}
                  onChange={(e) => setSymbol(e.target.value)}
                  className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
                >
                  <option value="">选择币种...</option>
                  {CRYPTO_SYMBOLS.map((c) => (
                    <option key={c.symbol} value={c.symbol}>
                      {c.name} ({c.symbol})
                    </option>
                  ))}
                </select>
              ) : (
                <input
                  type="text"
                  placeholder={
                    market === "US"
                      ? "如 VTI, SPY"
                      : market === "CN"
                        ? "如 510300, 600519"
                        : "如 2800, 9988"
                  }
                  value={symbol}
                  onChange={(e) => setSymbol(e.target.value)}
                  className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
                />
              )}
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                名称 (选填)
              </label>
              <input
                type="text"
                placeholder="如: 沪深300"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
              />
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                {market === "COMMODITY" ? "买入单价 (元/克, 选填)" : "买入单价 (选填, 货币)"}
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
                  placeholder="均价"
                  value={costPrice}
                  onChange={(e) => setCostPrice(e.target.value)}
                  className="w-full px-3 py-2 border border-[#E9ECEF] rounded-r-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono min-w-0"
                />
              </div>
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                {market === "COMMODITY"
                  ? "数量 (克)"
                  : market === "CRYPTO"
                    ? "数量"
                    : "份额 (股份)"}
              </label>
              <input
                type="number"
                placeholder={
                  market === "COMMODITY"
                    ? "如: 50"
                    : market === "CRYPTO"
                      ? "如: 0.5"
                      : "0"
                }
                value={shares}
                onChange={(e) => setShares(e.target.value)}
                className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
              />
            </div>
          </>
        ) : (
          <>
            <div className="flex flex-col gap-1 md:col-span-2">
              <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                名称
              </label>
              <input
                type="text"
                placeholder={
                  assetId === "gold"
                    ? "如: 实物黄金 50g"
                    : assetId === "cash"
                      ? "如: 货币基金"
                      : "如: 某理财产品"
                }
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
              />
            </div>
            <div className="flex flex-col gap-1 md:col-span-1">
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
              <div className="flex flex-col gap-1 md:col-span-1">
                <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                  总成本 (选填)
                </label>
                <input
                  type="number"
                  placeholder="投入本金"
                  value={cost}
                  onChange={(e) => setCost(e.target.value)}
                  className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
                />
              </div>
            ) : (
              <>
                <div className="flex flex-col gap-1 md:col-span-1">
                  <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                    单价
                  </label>
                  <input
                    type="number"
                    placeholder="买入单价"
                    value={manualPrice}
                    onChange={(e) => setManualPrice(e.target.value)}
                    className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
                  />
                </div>
                <div className="flex flex-col gap-1 md:col-span-1">
                  <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                    份额
                  </label>
                  <input
                    type="number"
                    placeholder="数量"
                    value={manualShares}
                    onChange={(e) => setManualShares(e.target.value)}
                    className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
                  />
                </div>
              </>
            )}
            <div className="flex flex-col gap-1 md:col-span-1">
              <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                当前总市值
              </label>
              <input
                type="number"
                placeholder="最新估值"
                value={value}
                onChange={(e) => setValue(e.target.value)}
                className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
              />
            </div>
          </>
        )}

        <div className="flex flex-col gap-1">
          <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
            手续费 (选填)
          </label>
          <input
            type="number"
            placeholder="0"
            value={fee}
            onChange={(e) => setFee(e.target.value)}
            className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
          />
        </div>

        <div className="flex flex-col justify-end gap-2">
          <label className="flex items-center gap-2 cursor-pointer pb-1 h-5.25">
            <input
              type="checkbox"
              checked={deductFromCash}
              onChange={(e) => setDeductFromCash(e.target.checked)}
              className="rounded border-[#E9ECEF] text-[#1A1A1A] focus:ring-[#1A1A1A]"
            />
            <span className="text-[10px] uppercase tracking-widest text-[#495057] font-bold">
              从可用资金扣除
            </span>
          </label>
          <button
            onClick={handleAdd}
            disabled={isFetching}
            className="w-full bg-[#1A1A1A] text-white py-2 rounded-lg text-sm font-medium hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {isFetching ? "获取中..." : "确认添加"}
          </button>
        </div>
      </div>
    </div>
  )
}
