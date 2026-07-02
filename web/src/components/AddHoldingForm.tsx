import { useState } from "react"
import { AssetId, ASSET_DEFINITIONS, COMMODITY_CN_SYMBOLS, COMMODITY_INTL_SYMBOLS, CRYPTO_SYMBOLS } from "../types"
import * as api from "../api"
import { useToast } from "./toast-context"

interface AddHoldingFormProps {
  onAddHolding: (holding: Omit<import("../types").Holding, "id">) => Promise<void>
  onClose: () => void
}

export default function AddHoldingForm({ onAddHolding, onClose }: AddHoldingFormProps) {
  const [assetId, setAssetId] = useState<AssetId>("stocks")
  const [market, setMarket] = useState<"US" | "CN" | "HK" | "FUND" | "COMMODITY_CN" | "COMMODITY_INTL" | "CRYPTO">("US")
  const [symbol, setSymbol] = useState("")
  const [name, setName] = useState("")
  const [isManual, setIsManual] = useState(false)
  const [isFetching, setIsFetching] = useState(false)

  const { showToast } = useToast()

  const handleAdd = async () => {
    if (isManual) {
      const targetCurrency =
        market === "US" || market === "CRYPTO" ? "USD" : market === "HK" ? "HKD" : "CNY"
      try {
        await onAddHolding({
          assetId,
          symbol: "",
          name: name.trim() || "手工资产",
          market,
          currency: targetCurrency,
          shares: 0,
          price: 0,
          value: 0,
        })
      } catch (e) {
        showToast(e instanceof Error ? e.message : "录入失败", "error")
        return
      }
    } else {
      if (!symbol) {
        showToast("请输入股票/基金代码", "error")
        return
      }

      setIsFetching(true)
      try {
        const data = await api.fetchPrice(symbol, market)
        if (data && data.price) {
          const authoritativeSymbol = data.symbol || symbol.toUpperCase()
          const targetCurrency =
            market === "US" || market === "CRYPTO" || market === "COMMODITY_INTL" ? "USD" : market === "HK" ? "HKD" : "CNY"

          await onAddHolding({
            assetId,
            symbol: authoritativeSymbol,
            name: name.trim() || data.name || authoritativeSymbol,
            market,
            currency: targetCurrency,
            shares: 0,
            price: data.price,
            value: 0,
          })
        } else {
          showToast("价格获取失败，请尝试手动录入", "error")
          setIsFetching(false)
          return
        }
      } catch (e) {
        showToast("价格获取失败，请尝试手动录入", "error")
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

        {!isManual ? (
          <>
            <div className="flex flex-col gap-1">
              <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                市场
              </label>
              <select
                value={market}
                onChange={(e) => {
                  const m = e.target.value as "US" | "CN" | "HK" | "FUND" | "COMMODITY_CN" | "COMMODITY_INTL" | "CRYPTO"
                  setMarket(m)
                  setSymbol("")
                }}
                className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
              >
                <option value="US">美股</option>
                <option value="CN">A股</option>
                <option value="HK">港股</option>
                <option value="FUND">场外基金</option>
                <option value="COMMODITY_CN">商品 (国内)</option>
                <option value="COMMODITY_INTL">商品 (国际)</option>
                <option value="CRYPTO">加密货币</option>
              </select>
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                {(market === "COMMODITY_CN" || market === "COMMODITY_INTL") ? "商品" : market === "CRYPTO" ? "币种" : "代码"}
              </label>
              {market === "COMMODITY_CN" ? (
                <select
                  value={symbol}
                  onChange={(e) => setSymbol(e.target.value)}
                  className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
                >
                  <option value="">选择商品...</option>
                  {COMMODITY_CN_SYMBOLS.map((c) => (
                    <option key={c.symbol} value={c.symbol}>
                      {c.name} ({c.symbol})
                    </option>
                  ))}
                </select>
              ) : market === "COMMODITY_INTL" ? (
                <select
                  value={symbol}
                  onChange={(e) => setSymbol(e.target.value)}
                  className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
                >
                  <option value="">选择商品...</option>
                  {COMMODITY_INTL_SYMBOLS.map((c) => (
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
                        : market === "FUND"
                          ? "如 110011, 161725"
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
          </>
        ) : (
          <div className="flex flex-col gap-1 md:col-span-2">
            <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
              名称
            </label>
            <input
              type="text"
              placeholder={
                assetId === "commodities"
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
        )}

        <div className="flex flex-col justify-end gap-2">
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
