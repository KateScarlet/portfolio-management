import { Account } from "../types"

interface Props {
  accounts: Account[]
  current: Account | null
  onSelect: (a: Account | null) => void
  onManage: () => void
}

export default function AccountSelector({ accounts, current, onSelect, onManage }: Props) {
  return (
    <div className="flex items-center gap-2">
      <select
        value={current?.id || ""}
        onChange={(e) => {
          if (e.target.value === "") {
            onSelect(null)
          } else {
            const a = accounts.find((a) => a.id === e.target.value)
            if (a) onSelect(a)
          }
        }}
        className="text-sm border border-[#E9ECEF] rounded-md px-2 py-1 bg-white text-[#1A1A1A] focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
      >
        <option value="">全部账户</option>
        {accounts.map((a) => (
          <option key={a.id} value={a.id}>
            {a.name}
            {a.broker ? ` (${a.broker})` : ""}
          </option>
        ))}
      </select>
      <button
        onClick={onManage}
        className="text-xs text-[#6C757D] hover:text-[#1A1A1A] transition-colors"
        title="管理账户"
      >
        管理
      </button>
    </div>
  )
}
