import { useState } from "react"
import { Holding, HoldingLot } from "../types"

interface LotEditorProps {
  holding: Holding
  lot: HoldingLot
  onSave: (updates: Partial<Holding>) => void
  onCancel: () => void
}

export default function LotEditor({ holding, lot, onSave, onCancel }: LotEditorProps) {
  const [date, setDate] = useState(new Date(lot.date).toISOString().split("T")[0])
  const [shares, setShares] = useState(lot.shares.toString())
  const [costPrice, setCostPrice] = useState((lot.costPrice || 0).toString())
  const [cost, setCost] = useState(
    (
      holding.symbol
        ? (lot.cost ?? 0)
        : lot.valueAdded || lot.cost || 0
    ).toString()
  )
  const [fee, setFee] = useState((lot.fee || 0).toString())

  const handleSave = () => {
    const dateVal = new Date(date).getTime() || lot.date
    const feeNum = parseFloat(fee) || 0

    if (holding.symbol) {
      const sharesNum = parseFloat(shares) || lot.shares
      const costPriceNum = parseFloat(costPrice) || lot.costPrice || 0
      const costNum = parseFloat(cost) || sharesNum * costPriceNum
      onSave({ lots: recalcLots({ ...lot, date: dateVal, shares: sharesNum, costPrice: costPriceNum, cost: costNum, fee: feeNum }) })
    } else {
      const valueAdded = parseFloat(cost) || lot.valueAdded || 0
      onSave({ lots: recalcLots({ ...lot, date: dateVal, valueAdded, cost: valueAdded, fee: feeNum }) })
    }
  }

  function recalcLots(updatedLot: HoldingLot): HoldingLot[] {
    if (!holding.lots) return []
    const newLots = holding.lots.map((l) => (l.id === updatedLot.id ? updatedLot : l))
    return recalcFromLots(newLots)
  }

  function recalcFromLots(lots: HoldingLot[]): HoldingLot[] {
    return lots
  }

  return (
    <div className="flex flex-wrap items-center gap-2 w-full">
      <input
        type="date"
        value={date}
        onChange={(e) => setDate(e.target.value)}
        className="px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A]"
      />
      {holding.symbol ? (
        <>
          <input
            type="number"
            placeholder="单价"
            value={costPrice}
            onChange={(e) => setCostPrice(e.target.value)}
            className="w-20 px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A] font-mono"
          />
          <input
            type="number"
            placeholder="数量"
            value={shares}
            onChange={(e) => setShares(e.target.value)}
            className="w-20 px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A] font-mono"
          />
        </>
      ) : null}
      <input
        type="number"
        placeholder={holding.symbol ? "成本" : "价值"}
        value={cost}
        onChange={(e) => setCost(e.target.value)}
        className="w-24 px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A] font-mono"
      />
      <input
        type="number"
        placeholder="手续费"
        value={fee}
        onChange={(e) => setFee(e.target.value)}
        className="w-20 px-2 py-1 border border-[#E9ECEF] rounded text-xs focus:outline-none focus:border-[#1A1A1A] font-mono"
      />
      <div className="flex gap-1 ml-auto">
        <button
          onClick={handleSave}
          className="text-[10px] text-white bg-[#1A1A1A] px-2 py-1 rounded hover:opacity-90"
        >
          保存
        </button>
        <button
          onClick={onCancel}
          className="text-[10px] text-[#ADB5BD] hover:text-[#1A1A1A] px-1"
        >
          取消
        </button>
      </div>
    </div>
  )
}
