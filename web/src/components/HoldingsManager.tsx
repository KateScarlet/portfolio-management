import React, { useState } from "react"
import { AssetId, ASSET_DEFINITIONS, Holding, COMMODITY_SYMBOLS, CRYPTO_SYMBOLS } from "../types"
import { formatCurrency, formatPercent } from "../utils"
import * as api from "../api"

interface HoldingsManagerProps {
  holdings: Holding[]
  setHoldings: React.Dispatch<React.SetStateAction<Holding[]>>
  total: number
  onAddHolding: (holding: Omit<Holding, "id">) => void
  onUpdateHolding: (id: string, updates: Partial<Holding>) => void
  onRemoveHolding: (id: string) => void
  onSaveRecord: () => void
}

export default function HoldingsManager({
  holdings,
  setHoldings,
  total: _total,
  onAddHolding,
  onUpdateHolding,
  onRemoveHolding,
  onSaveRecord,
}: HoldingsManagerProps) {
  const [isAdding, setIsAdding] = useState(false)
  const [newAssetId, setNewAssetId] = useState<AssetId>("stocks")
  const [newMarket, setNewMarket] = useState<"US" | "CN" | "HK" | "COMMODITY" | "CRYPTO">("US")
  const [newSymbol, setNewSymbol] = useState("")
  const [newName, setNewName] = useState("")
  const [newShares, setNewShares] = useState("")
  const [newCostPrice, setNewCostPrice] = useState("")
  const [costCurrency, setCostCurrency] = useState("CNY")
  const [newValue, setNewValue] = useState("")
  const [newCost, setNewCost] = useState("")
  const [newFee, setNewFee] = useState("")
  const [newDate, setNewDate] = useState(new Date().toISOString().split("T")[0])
  const [isManual, setIsManual] = useState(false)
  const [isFetching, setIsFetching] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [editingValueId, setEditingValueId] = useState<string | null>(null)
  const [tempEditValue, setTempEditValue] = useState("")
  const [editingLotId, setEditingLotId] = useState<string | null>(null)
  const [tempEditLot, setTempEditLot] = useState<{
    date: string
    shares: string
    costPrice: string
    cost: string
    fee: string
  }>({ date: "", shares: "", costPrice: "", cost: "", fee: "" })

  const [sellingHolding, setSellingHolding] = useState<Holding | null>(null)
  const [sellShares, setSellShares] = useState("")
  const [sellPrice, setSellPrice] = useState("")
  const [sellFee, setSellFee] = useState("")
  const [deductFromCash, setDeductFromCash] = useState(false)

  const handleAdd = async () => {
    const feeNum = parseFloat(newFee) || 0
    let addedCost: number | undefined

    if (isManual) {
      const val = parseFloat(newValue)
      const c = parseFloat(newCost)
      if (isNaN(val) || val <= 0) return
      addedCost = (isNaN(c) || c <= 0 ? val : c) + feeNum
      onAddHolding({
        assetId: newAssetId,
        symbol: "",
        name: newName.trim() || "手工资产",
        shares: 0,
        price: 0,
        value: val,
        cost: addedCost - feeNum,
        fee: feeNum,
        date: new Date(newDate).getTime(),
      })
    } else {
      const sharesNum = parseFloat(newShares)
      const cPrice = parseFloat(newCostPrice)
      if (isNaN(sharesNum) || sharesNum <= 0 || !newSymbol) return

      setIsFetching(true)
      try {
        const res = await fetch(`/api/price/${newSymbol}?targetCurrency=CNY`)
        const data = await res.json()
        if (data && data.price) {
          const authoritativeSymbol = data.symbol || newSymbol.toUpperCase()
          let finalCostPrice = isNaN(cPrice) || cPrice <= 0 ? data.price : cPrice

          if (!isNaN(cPrice) && cPrice > 0 && costCurrency !== "CNY") {
            try {
              const fxRes = await fetch(`/api/exchange/${costCurrency}CNY`)
              const fxData = await fxRes.json()
              if (fxData && fxData.rate) {
                finalCostPrice = cPrice * fxData.rate
              }
            } catch {
              console.error("Failed to fetch exchange rate")
            }
          }

          addedCost = sharesNum * finalCostPrice + feeNum
          onAddHolding({
            assetId: newAssetId,
            symbol: authoritativeSymbol,
            name: newName.trim() || data.name || authoritativeSymbol,
            shares: sharesNum,
            price: data.price,
            costPrice: finalCostPrice,
            value: sharesNum * data.price,
            cost: sharesNum * finalCostPrice,
            fee: feeNum,
            date: new Date(newDate).getTime(),
          })
        } else {
          alert("Failed to fetch price. Try manual entry.")
          setIsFetching(false)
          return
        }
      } catch {
        alert("Failed to fetch price. Try manual entry.")
        setIsFetching(false)
        return
      } finally {
        setIsFetching(false)
      }
    }

    if (deductFromCash && addedCost && addedCost > 0) {
      const cashHolding = holdings.find((hd) => hd.assetId === "cash")
      if (cashHolding) {
        const remainingValue = Math.max(0, cashHolding.value - addedCost)
        onUpdateHolding(cashHolding.id, {
          value: remainingValue,
          cost:
            (cashHolding.cost || cashHolding.value) - addedCost > 0
              ? (cashHolding.cost || cashHolding.value) - addedCost
              : remainingValue,
        })
      }
    }

    setIsAdding(false)
    setNewSymbol("")
    setNewName("")
    setNewShares("")
    setNewCostPrice("")
    setNewValue("")
    setNewCost("")
    setNewFee("")
    setDeductFromCash(false)
  }

  const syncAllPrices = async () => {
    setSyncing(true)
    try {
      await api.triggerSync()
    } finally {
      setSyncing(false)
    }
  }

  const recalcHolding = (h: Holding, lots: typeof h.lots) => {
    if (!lots) return {}
    const buyLots = lots.filter((l) => l.type !== "sell")
    const isSymbol = !!h.symbol
    if (isSymbol) {
      const totalShares = buyLots.reduce((s, l) => s + l.shares, 0)
      const totalCost = buyLots.reduce((s, l) => s + (l.cost || 0) + (l.fee || 0), 0)
      const costPrice = totalShares > 0 ? totalCost / totalShares : 0
      return { lots, shares: totalShares, cost: totalCost, costPrice }
    } else {
      const totalValue = buyLots.reduce((s, l) => s + (l.valueAdded || l.cost || 0), 0)
      const totalCost = buyLots.reduce(
        (s, l) => s + (l.cost || l.valueAdded || 0) + (l.fee || 0),
        0
      )
      return { lots, value: totalValue, cost: totalCost, shares: 0 }
    }
  }

  const saveEditLot = (h: Holding) => {
    if (!editingLotId || !h.lots) return
    const lot = h.lots.find((l) => l.id === editingLotId)
    if (!lot) return
    const updatedLots = h.lots.map((l) => {
      if (l.id !== editingLotId) return l
      const dateVal = new Date(tempEditLot.date).getTime() || l.date
      const fee = parseFloat(tempEditLot.fee) || 0
      if (h.symbol) {
        const shares = parseFloat(tempEditLot.shares) || l.shares
        const costPrice = parseFloat(tempEditLot.costPrice) || l.costPrice || 0
        const cost = parseFloat(tempEditLot.cost) || shares * costPrice
        return { ...l, date: dateVal, shares, costPrice, cost, fee }
      } else {
        const valueAdded = parseFloat(tempEditLot.cost) || l.valueAdded || 0
        const cost = valueAdded
        return { ...l, date: dateVal, valueAdded, cost, fee }
      }
    })
    onUpdateHolding(h.id, recalcHolding(h, updatedLots))
    setEditingLotId(null)
  }

  const deleteEditLot = (h: Holding, lotId: string) => {
    if (!h.lots) return
    const updatedLots = h.lots.filter((l) => l.id !== lotId)
    onUpdateHolding(h.id, recalcHolding(h, updatedLots))
    setEditingLotId(null)
  }

  const handleSell = (h: Holding) => {
    setSellingHolding(h)
    if (h.shares && h.shares > 0 && h.price) {
      setSellShares(h.shares.toString())
      setSellPrice(h.price.toString())
    } else {
      setSellShares("")
      setSellPrice(h.value.toString())
    }
  }

  const confirmSell = async () => {
    if (!sellingHolding) return
    const h = sellingHolding
    const feeNum = parseFloat(sellFee) || 0

    if (h.shares && h.shares > 0 && h.price) {
      const sShares = parseFloat(sellShares)
      const sPrice = parseFloat(sellPrice)
      if (isNaN(sShares) || sShares <= 0 || sShares > h.shares) return
      if (isNaN(sPrice) || sPrice < 0) return

      try {
        const result = await api.sellHolding(h.id, sShares, sPrice, feeNum, 0)
        setHoldings(result.holdings)
      } catch (e) {
        console.error("Failed to sell holding", e)
        alert("卖出失败，请重试")
        return
      }
    } else {
      const sValue = parseFloat(sellPrice)
      if (isNaN(sValue) || sValue <= 0 || sValue > h.value) return

      try {
        const result = await api.sellHolding(h.id, 0, 0, feeNum, sValue)
        setHoldings(result.holdings)
      } catch (e) {
        console.error("Failed to sell holding", e)
        alert("卖出失败，请重试")
        return
      }
    }

    setSellingHolding(null)
    setSellFee("")
  }

  return (
    <div className="bg-white rounded-2xl border border-[#E9ECEF] shadow-sm flex flex-col overflow-hidden">
      <div className="p-6 border-b border-[#F1F3F5] flex justify-between items-center bg-white flex-wrap gap-4">
        <h3 className="text-lg font-medium text-[#1A1A1A]">资产明细</h3>
        <div className="flex gap-2">
          <button
            onClick={syncAllPrices}
            disabled={syncing}
            className="text-xs bg-[#F8F9FA] border border-[#DEE2E6] text-[#1A1A1A] px-4 py-2 rounded-full hover:bg-gray-50 font-medium transition-colors disabled:opacity-50"
          >
            {syncing ? "同步中..." : "同步价格"}
          </button>
          <button
            onClick={onSaveRecord}
            className="text-xs bg-[#F8F9FA] border border-[#DEE2E6] text-[#1A1A1A] px-4 py-2 rounded-full hover:bg-gray-50 font-medium transition-colors"
          >
            保存记录
          </button>
          <button
            onClick={() => setIsAdding(!isAdding)}
            className="text-xs bg-[#1A1A1A] text-white px-4 py-2 rounded-full hover:opacity-90 transition-opacity"
          >
            {isAdding ? "取消" : "+ 录入资产"}
          </button>
        </div>
      </div>

      {isAdding && (
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
              自动获取价格 (股票/ETF/基金)
            </label>
            <label className="flex items-center gap-2 text-sm text-[#495057]">
              <input
                type="radio"
                checked={isManual}
                onChange={() => setIsManual(true)}
                className="text-[#1A1A1A] focus:ring-[#1A1A1A]"
              />
              手动录入价值
            </label>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 xl:grid-cols-8 gap-4">
            <div className="flex flex-col gap-1">
              <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                分类
              </label>
              <select
                value={newAssetId}
                onChange={(e) => setNewAssetId(e.target.value as AssetId)}
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
                value={newDate}
                onChange={(e) => setNewDate(e.target.value)}
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
                    value={newMarket}
                    onChange={(e) => {
                      setNewMarket(e.target.value as "US" | "CN" | "HK" | "COMMODITY" | "CRYPTO")
                      setNewSymbol("")
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
                    {newMarket === "COMMODITY" ? "商品" : newMarket === "CRYPTO" ? "币种" : "代码"}
                  </label>
                  {newMarket === "COMMODITY" ? (
                    <select
                      value={newSymbol}
                      onChange={(e) => setNewSymbol(e.target.value)}
                      className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
                    >
                      <option value="">选择商品...</option>
                      {COMMODITY_SYMBOLS.map((c) => (
                        <option key={c.symbol} value={c.symbol}>
                          {c.name} ({c.symbol})
                        </option>
                      ))}
                    </select>
                  ) : newMarket === "CRYPTO" ? (
                    <select
                      value={newSymbol}
                      onChange={(e) => setNewSymbol(e.target.value)}
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
                        newMarket === "US"
                          ? "如 VTI, SPY"
                          : newMarket === "CN"
                            ? "如 510300, 600519"
                            : "如 2800, 9988"
                      }
                      value={newSymbol}
                      onChange={(e) => setNewSymbol(e.target.value)}
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
                    value={newName}
                    onChange={(e) => setNewName(e.target.value)}
                    className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
                  />
                </div>
                <div className="flex flex-col gap-1">
                  <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                    {newMarket === "COMMODITY" ? "买入单价 (元/克, 选填)" : "买入单价 (选填, 货币)"}
                  </label>
                  <div className="flex w-full">
                    <select
                      value={costCurrency}
                      onChange={(e) => setCostCurrency(e.target.value)}
                      className="px-2 py-2 border border-r-0 border-[#E9ECEF] rounded-l-lg text-xs bg-gray-50 focus:outline-none focus:border-[#1A1A1A] w-[70px]"
                    >
                      <option value="CNY">CNY</option>
                      <option value="USD">USD</option>
                      <option value="HKD">HKD</option>
                    </select>
                    <input
                      type="number"
                      placeholder="均价"
                      value={newCostPrice}
                      onChange={(e) => setNewCostPrice(e.target.value)}
                      className="w-full px-3 py-2 border border-[#E9ECEF] rounded-r-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono min-w-0"
                    />
                  </div>
                </div>
                <div className="flex flex-col gap-1">
                  <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                    {newMarket === "COMMODITY"
                      ? "数量 (克)"
                      : newMarket === "CRYPTO"
                        ? "数量"
                        : "份额 (股份)"}
                  </label>
                  <input
                    type="number"
                    placeholder={
                      newMarket === "COMMODITY"
                        ? "如: 50"
                        : newMarket === "CRYPTO"
                          ? "如: 0.5"
                          : "0"
                    }
                    value={newShares}
                    onChange={(e) => setNewShares(e.target.value)}
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
                      newAssetId === "gold"
                        ? "如: 实物黄金 50g"
                        : newAssetId === "cash"
                          ? "如: 货币基金"
                          : "如: 某理财产品"
                    }
                    value={newName}
                    onChange={(e) => setNewName(e.target.value)}
                    className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A]"
                  />
                </div>
                <div className="flex flex-col gap-1 md:col-span-1">
                  <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                    总成本 (选填)
                  </label>
                  <input
                    type="number"
                    placeholder="投入本金"
                    value={newCost}
                    onChange={(e) => setNewCost(e.target.value)}
                    className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
                  />
                </div>
                <div className="flex flex-col gap-1 md:col-span-1">
                  <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                    当前总市值
                  </label>
                  <input
                    type="number"
                    placeholder="最新估值"
                    value={newValue}
                    onChange={(e) => setNewValue(e.target.value)}
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
                value={newFee}
                onChange={(e) => setNewFee(e.target.value)}
                className="w-full px-3 py-2 border border-[#E9ECEF] rounded-lg text-sm bg-white focus:outline-none focus:border-[#1A1A1A] font-mono"
              />
            </div>

            <div className="flex flex-col justify-end gap-2">
              <label className="flex items-center gap-2 cursor-pointer pb-1 h-[21px]">
                <input
                  type="checkbox"
                  checked={deductFromCash}
                  onChange={(e) => setDeductFromCash(e.target.checked)}
                  className="rounded border-[#E9ECEF] text-[#1A1A1A] focus:ring-[#1A1A1A]"
                />
                <span className="text-[10px] uppercase tracking-widest text-[#495057] font-bold">
                  从现金扣除本金
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
      )}

      <div className="flex-grow overflow-x-auto">
        <table className="w-full text-left">
          <thead className="text-[10px] uppercase tracking-widest text-[#ADB5BD] border-b border-[#F1F3F5] bg-white">
            <tr>
              <th className="px-6 py-4 font-bold">资产大类</th>
              <th className="px-6 py-4 font-bold">代码/名称</th>
              <th className="px-6 py-4 font-bold text-right">单价 & 份额</th>
              <th className="px-6 py-4 font-bold text-right">总成本 & 盈亏</th>
              <th className="px-6 py-4 font-bold text-right">当前总市值</th>
              <th className="px-6 py-4 font-bold text-right">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[#F8F9FA] bg-white text-[#1A1A1A]">
            {holdings.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-6 py-12 text-center text-sm text-[#ADB5BD]">
                  暂无资产明细，请在上方点击"录入资产"开始构建组合。
                </td>
              </tr>
            ) : (
              holdings.map((h) => {
                const def = ASSET_DEFINITIONS[h.assetId]
                const isExpanded = expandedId === h.id

                return (
                  <React.Fragment key={h.id}>
                    <tr
                      className="hover:bg-[#F8F9FA] transition-colors group cursor-pointer"
                      onClick={() => setExpandedId(isExpanded ? null : h.id)}
                    >
                      <td className="px-6 py-5 flex items-center gap-3">
                        <div
                          className={`w-8 h-8 rounded flex items-center justify-center text-[10px] font-bold ${h.assetId === "cash" ? "text-[#495057] border border-[#DEE2E6]" : "text-white"}`}
                          style={{
                            backgroundColor: def.color,
                          }}
                        >
                          {h.assetId === "stocks"
                            ? "STK"
                            : h.assetId === "bonds"
                              ? "BND"
                              : h.assetId === "gold"
                                ? "CMD"
                                : "CSH"}
                        </div>
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
                            <p
                              className="text-[10px] text-[#ADB5BD] truncate max-w-[150px]"
                              title={h.name}
                            >
                              {h.name}
                            </p>
                          </div>
                        ) : (
                          <div className="flex items-center gap-2">
                            <p className="text-sm text-[#6C757D]">{h.name || "手工资产"}</p>
                            {h.lots && h.lots.length > 0 && (
                              <span className="text-[10px] bg-gray-100 text-gray-500 px-1.5 py-0.5 rounded">
                                {h.lots.length} 笔
                              </span>
                            )}
                          </div>
                        )}
                      </td>
                      <td className="px-6 py-5 text-right font-mono text-sm text-[#495057]">
                        {h.symbol ? (
                          <div>
                            <p>{formatCurrency(h.price)}</p>
                            <p className="text-[10px] text-[#ADB5BD]">× {h.shares}</p>
                          </div>
                        ) : (
                          <span className="text-[#ADB5BD] text-xs">-</span>
                        )}
                      </td>
                      <td className="px-6 py-5 text-right font-mono text-sm text-[#495057]">
                        {h.cost && h.cost > 0 ? (
                          <div>
                            <p>{formatCurrency(h.cost)}</p>
                            {(() => {
                              const profit = h.value - h.cost
                              const returnRate = profit / h.cost
                              const isPositive = profit >= 0
                              return (
                                <p
                                  className={`text-[10px] ${isPositive ? "text-emerald-600" : "text-orange-600"}`}
                                >
                                  {isPositive ? "+" : ""}
                                  {formatPercent(returnRate)}
                                </p>
                              )
                            })()}
                          </div>
                        ) : (
                          <span className="text-[#ADB5BD] text-xs">-</span>
                        )}
                      </td>
                      <td
                        className="px-6 py-5 text-right font-medium text-sm font-mono"
                        onClick={(e) => e.stopPropagation()}
                      >
                        {editingValueId === h.id ? (
                          <div className="flex items-center justify-end gap-2">
                            <input
                              type="number"
                              value={tempEditValue}
                              onChange={(e) => setTempEditValue(e.target.value)}
                              className="w-24 px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A] font-mono text-right"
                              autoFocus
                            />
                            <button
                              onClick={() => {
                                const num = parseFloat(tempEditValue)
                                if (!isNaN(num) && num >= 0) {
                                  onUpdateHolding(h.id, {
                                    value: num,
                                  })
                                  setEditingValueId(null)
                                }
                              }}
                              className="text-[10px] text-white bg-[#1A1A1A] px-2 py-1 rounded hover:opacity-90"
                            >
                              保存
                            </button>
                            <button
                              onClick={() => setEditingValueId(null)}
                              className="text-[10px] text-[#ADB5BD] hover:text-[#1A1A1A]"
                            >
                              取消
                            </button>
                          </div>
                        ) : (
                          formatCurrency(h.value)
                        )}
                      </td>
                      <td
                        className="px-6 py-5 text-right space-x-2"
                        onClick={(e) => e.stopPropagation()}
                      >
                        {!h.symbol && (
                          <button
                            onClick={() => {
                              setEditingValueId(h.id)
                              setTempEditValue(h.value.toString())
                            }}
                            className="text-[10px] uppercase tracking-wider text-[#1A1A1A] hover:text-blue-600 font-bold transition-colors"
                            title="更新估值"
                          >
                            Update
                          </button>
                        )}
                        {h.assetId !== "cash" && (
                          <button
                            onClick={() => handleSell(h)}
                            className="text-[10px] uppercase tracking-wider text-[#1A1A1A] hover:text-orange-500 font-bold transition-colors"
                            title="卖出"
                          >
                            Sell
                          </button>
                        )}
                        <button
                          onClick={() => onRemoveHolding(h.id)}
                          className="text-[10px] uppercase tracking-wider text-[#ADB5BD] hover:text-orange-500 font-bold transition-colors"
                        >
                          Del
                        </button>
                      </td>
                    </tr>

                    {isExpanded && h.lots && h.lots.length > 0 && (
                      <tr className="bg-[#F8F9FA]">
                        <td colSpan={6} className="px-6 py-4">
                          <div className="pl-12 border-l-2 border-[#DEE2E6] ml-4 space-y-2">
                            <h5 className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold mb-2">
                              购买/录入记录
                            </h5>
                            {h.lots.map((lot) => {
                              const isEditing = editingLotId === lot.id
                              return (
                                <div
                                  key={lot.id}
                                  className={`flex justify-between items-center text-xs font-mono border-b border-[#E9ECEF] last:border-0 pb-2 last:pb-0 whitespace-nowrap ${isEditing ? "bg-white -mx-2 px-2 py-2 rounded-lg border border-[#DEE2E6] flex-wrap" : "text-[#495057]"} ${lot.type === "sell" ? "!text-orange-600" : ""}`}
                                >
                                  {isEditing ? (
                                    <div className="flex flex-wrap items-center gap-2 w-full">
                                      <input
                                        type="date"
                                        value={tempEditLot.date}
                                        onChange={(e) =>
                                          setTempEditLot({
                                            ...tempEditLot,
                                            date: e.target.value,
                                          })
                                        }
                                        className="px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A]"
                                      />
                                      {h.symbol ? (
                                        <>
                                          <input
                                            type="number"
                                            placeholder="单价"
                                            value={tempEditLot.costPrice}
                                            onChange={(e) =>
                                              setTempEditLot({
                                                ...tempEditLot,
                                                costPrice: e.target.value,
                                              })
                                            }
                                            className="w-20 px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A] font-mono"
                                          />
                                          <input
                                            type="number"
                                            placeholder="数量"
                                            value={tempEditLot.shares}
                                            onChange={(e) =>
                                              setTempEditLot({
                                                ...tempEditLot,
                                                shares: e.target.value,
                                              })
                                            }
                                            className="w-20 px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A] font-mono"
                                          />
                                        </>
                                      ) : null}
                                      <input
                                        type="number"
                                        placeholder={h.symbol ? "成本" : "价值"}
                                        value={tempEditLot.cost}
                                        onChange={(e) =>
                                          setTempEditLot({
                                            ...tempEditLot,
                                            cost: e.target.value,
                                          })
                                        }
                                        className="w-24 px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A] font-mono"
                                      />
                                      <input
                                        type="number"
                                        placeholder="手续费"
                                        value={tempEditLot.fee}
                                        onChange={(e) =>
                                          setTempEditLot({
                                            ...tempEditLot,
                                            fee: e.target.value,
                                          })
                                        }
                                        className="w-20 px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A] font-mono"
                                      />
                                      <div className="flex gap-1 ml-auto">
                                        <button
                                          onClick={() => saveEditLot(h)}
                                          className="text-[10px] text-white bg-[#1A1A1A] px-2 py-1 rounded hover:opacity-90"
                                        >
                                          保存
                                        </button>
                                        <button
                                          onClick={() => setEditingLotId(null)}
                                          className="text-[10px] text-[#ADB5BD] hover:text-[#1A1A1A] px-1"
                                        >
                                          取消
                                        </button>
                                      </div>
                                    </div>
                                  ) : (
                                    <>
                                      <span className="flex items-center gap-2">
                                        {lot.type === "sell" ? (
                                          <span className="text-[9px] bg-orange-100 text-orange-600 px-1.5 py-0.5 rounded font-bold">
                                            卖出
                                          </span>
                                        ) : (
                                          <span className="text-[9px] bg-blue-100 text-blue-600 px-1.5 py-0.5 rounded font-bold">
                                            买入
                                          </span>
                                        )}
                                        {new Date(lot.date).toLocaleDateString()}
                                      </span>
                                      <div className="flex items-center gap-4 text-right flex-shrink-0">
                                        {h.symbol ? (
                                          <>
                                            <span className="w-28 text-right">
                                              单价: {formatCurrency(lot.costPrice || 0)}
                                            </span>
                                            <span className="w-20 text-right">
                                              数量: {lot.shares}
                                            </span>
                                            <span className="w-28 font-medium text-[#1A1A1A] text-right">
                                              {lot.type === "sell" ? "收入" : "成本"}:{" "}
                                              {formatCurrency(lot.cost || 0)}
                                            </span>
                                          </>
                                        ) : (
                                          <span className="w-28 font-medium text-[#1A1A1A] text-right">
                                            {lot.type === "sell" ? "收入" : "价值"}:{" "}
                                            {formatCurrency(lot.valueAdded || lot.cost || 0)}
                                          </span>
                                        )}
                                        {(lot.fee || 0) > 0 && (
                                          <span className="w-20 text-[10px] text-[#ADB5BD] text-right">
                                            费: {formatCurrency(lot.fee || 0)}
                                          </span>
                                        )}
                                        {lot.type !== "sell" && (
                                          <div className="flex gap-2 flex-shrink-0">
                                            <button
                                              onClick={() => {
                                                setEditingLotId(lot.id)
                                                setTempEditLot({
                                                  date: new Date(lot.date)
                                                    .toISOString()
                                                    .split("T")[0],
                                                  shares: lot.shares.toString(),
                                                  costPrice: (lot.costPrice || 0).toString(),
                                                  cost: (h.symbol
                                                    ? (lot.cost ?? 0)
                                                    : lot.valueAdded || lot.cost || 0
                                                  ).toString(),
                                                  fee: (lot.fee || 0).toString(),
                                                })
                                              }}
                                              className="text-[10px] uppercase tracking-wider text-[#1A1A1A] hover:text-blue-600 font-bold transition-colors whitespace-nowrap"
                                            >
                                              Edit
                                            </button>
                                            <button
                                              onClick={() => deleteEditLot(h, lot.id)}
                                              className="text-[10px] uppercase tracking-wider text-[#ADB5BD] hover:text-orange-500 font-bold transition-colors whitespace-nowrap"
                                            >
                                              Del
                                            </button>
                                          </div>
                                        )}
                                      </div>
                                    </>
                                  )}
                                </div>
                              )
                            })}
                          </div>
                        </td>
                      </tr>
                    )}
                  </React.Fragment>
                )
              })
            )}
          </tbody>
        </table>
      </div>

      {sellingHolding && (
        <div className="fixed inset-0 bg-[#1A1A1A]/80 z-50 flex items-center justify-center p-4 backdrop-blur-sm">
          <div className="bg-white rounded-2xl max-w-sm w-full p-6 shadow-2xl flex flex-col gap-6">
            <div>
              <h3 className="text-lg font-bold text-[#1A1A1A]">卖出资产</h3>
              <p className="text-sm text-[#6C757D] mt-1">
                {sellingHolding.name || sellingHolding.symbol}
              </p>
            </div>

            <div className="space-y-4">
              {sellingHolding.shares && sellingHolding.shares > 0 ? (
                <>
                  <div className="flex flex-col gap-2">
                    <label className="text-[10px] uppercase tracking-widest text-[#ADB5BD] font-bold">
                      卖出份额 (最多: {sellingHolding.shares})
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
                      卖出单价 (参考: {sellingHolding.price || 0})
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
                    卖出金额 (最多: {sellingHolding.value})
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
                onClick={() => {
                  setSellingHolding(null)
                  setSellFee("")
                }}
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
      )}
    </div>
  )
}
