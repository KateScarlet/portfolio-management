import { useState } from "react"
import { Portfolio } from "../types"
import * as api from "../api"

interface Props {
  portfolios: Portfolio[]
  onClose: () => void
  onRefresh: () => void
}

export default function PortfolioManager({ portfolios, onClose, onRefresh }: Props) {
  const [newName, setNewName] = useState("")
  const [newDesc, setNewDesc] = useState("")
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editName, setEditName] = useState("")
  const [editDesc, setEditDesc] = useState("")
  const [error, setError] = useState("")

  const handleCreate = async () => {
    if (!newName.trim()) return
    setError("")
    try {
      await api.createPortfolio(newName.trim(), newDesc.trim() || undefined)
      setNewName("")
      setNewDesc("")
      onRefresh()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "未知错误")
    }
  }

  const handleUpdate = async (id: string) => {
    if (!editName.trim()) return
    setError("")
    try {
      await api.updatePortfolio(id, {
        name: editName.trim(),
        description: editDesc.trim() || undefined,
      })
      setEditingId(null)
      onRefresh()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "未知错误")
    }
  }

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`确定删除组合"${name}"？该组合下的所有持仓和记录将被删除。`)) return
    setError("")
    try {
      await api.deletePortfolio(id)
      onRefresh()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "未知错误")
    }
  }

  return (
    <div
      className="fixed inset-0 bg-black/30 flex items-center justify-center z-50"
      onClick={onClose}
    >
      <div
        className="bg-white rounded-lg shadow-lg p-6 w-full max-w-md max-h-[80vh] overflow-y-auto"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">管理投资组合</h2>
          <button
            onClick={onClose}
            className="text-[#6C757D] hover:text-[#1A1A1A] text-xl leading-none"
          >
            &times;
          </button>
        </div>

        {error && <div className="mb-3 p-2 bg-red-50 text-red-700 text-xs rounded">{error}</div>}

        <div className="mb-4">
          <div className="flex gap-2 mb-2">
            <input
              type="text"
              placeholder="新组合名称"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              className="flex-1 text-sm border border-[#E9ECEF] rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
              onKeyDown={(e) => e.key === "Enter" && handleCreate()}
            />
            <button
              onClick={handleCreate}
              disabled={!newName.trim()}
              className="text-sm bg-[#1A1A1A] text-white px-3 py-1.5 rounded hover:bg-[#333] disabled:opacity-50 transition-colors"
            >
              创建
            </button>
          </div>
          <input
            type="text"
            placeholder="描述（可选）"
            value={newDesc}
            onChange={(e) => setNewDesc(e.target.value)}
            className="w-full text-sm border border-[#E9ECEF] rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
          />
        </div>

        <div className="space-y-2">
          {portfolios.map((p) => (
            <div key={p.id} className="border border-[#E9ECEF] rounded p-3">
              {editingId === p.id ? (
                <div className="space-y-2">
                  <input
                    type="text"
                    value={editName}
                    onChange={(e) => setEditName(e.target.value)}
                    className="w-full text-sm border border-[#E9ECEF] rounded px-2 py-1 focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
                  />
                  <input
                    type="text"
                    value={editDesc}
                    onChange={(e) => setEditDesc(e.target.value)}
                    placeholder="描述（可选）"
                    className="w-full text-sm border border-[#E9ECEF] rounded px-2 py-1 focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
                  />
                  <div className="flex gap-2">
                    <button
                      onClick={() => handleUpdate(p.id)}
                      className="text-xs bg-[#1A1A1A] text-white px-2 py-1 rounded"
                    >
                      保存
                    </button>
                    <button
                      onClick={() => setEditingId(null)}
                      className="text-xs text-[#6C757D] hover:text-[#1A1A1A]"
                    >
                      取消
                    </button>
                  </div>
                </div>
              ) : (
                <div className="flex items-center justify-between">
                  <div>
                    <span className="text-sm font-medium">{p.name}</span>
                    {p.isDefault && (
                      <span className="ml-2 text-[10px] text-[#6C757D] bg-[#F8F9FA] px-1.5 py-0.5 rounded">
                        默认
                      </span>
                    )}
                    {p.description && (
                      <p className="text-xs text-[#6C757D] mt-0.5">{p.description}</p>
                    )}
                  </div>
                  <div className="flex gap-2">
                    <button
                      onClick={() => {
                        setEditingId(p.id)
                        setEditName(p.name)
                        setEditDesc(p.description || "")
                      }}
                      className="text-xs text-[#6C757D] hover:text-[#1A1A1A]"
                    >
                      编辑
                    </button>
                    {!p.isDefault && (
                      <button
                        onClick={() => handleDelete(p.id, p.name)}
                        className="text-xs text-red-500 hover:text-red-700"
                      >
                        删除
                      </button>
                    )}
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
