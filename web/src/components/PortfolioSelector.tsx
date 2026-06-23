import { Portfolio } from "../types"

interface Props {
  portfolios: Portfolio[]
  current: Portfolio | null
  onSelect: (p: Portfolio) => void
  onManage: () => void
}

export default function PortfolioSelector({ portfolios, current, onSelect, onManage }: Props) {
  if (portfolios.length === 0) return null

  return (
    <div className="flex items-center gap-2">
      <select
        value={current?.id || ""}
        onChange={(e) => {
          const p = portfolios.find((p) => p.id === e.target.value)
          if (p) onSelect(p)
        }}
        className="text-sm border border-[#E9ECEF] rounded-md px-2 py-1 bg-white text-[#1A1A1A] focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
      >
        {portfolios.map((p) => (
          <option key={p.id} value={p.id}>
            {p.name}
          </option>
        ))}
      </select>
      <button
        onClick={onManage}
        className="text-xs text-[#6C757D] hover:text-[#1A1A1A] transition-colors"
        title="管理投资组合"
      >
        管理
      </button>
    </div>
  )
}
