import { ASSET_DEFINITIONS, AssetId } from "../types"

export default function AssetIcon({ assetId, size = 16 }: { assetId: AssetId; size?: number }) {
  const def = ASSET_DEFINITIONS[assetId]
  const Icon = def.icon
  return (
    <div
      className={`w-8 h-8 rounded flex items-center justify-center ${assetId === "cash" ? "text-[#495057] border border-[#DEE2E6]" : "text-white"}`}
      style={{ backgroundColor: def.color }}
    >
      <Icon size={size} strokeWidth={2} />
    </div>
  )
}
