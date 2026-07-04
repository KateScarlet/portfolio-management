import { useState } from "react"
import { Account } from "../types"
import * as api from "../api"
import ConfirmDialog from "./ConfirmDialog"

interface Props {
  accounts: Account[]
  onClose: () => void
  onRefresh: () => void
}

export default function AccountManager({ accounts, onClose, onRefresh }: Props) {
  const [newName, setNewName] = useState("")
  const [newDesc, setNewDesc] = useState("")
  const [newBroker, setNewBroker] = useState("")
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editName, setEditName] = useState("")
  const [editDesc, setEditDesc] = useState("")
  const [editBroker, setEditBroker] = useState("")
  const [error, setError] = useState("")
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null)

  const handleCreate = async () => {
    if (!newName.trim()) return
    setError("")
    try {
      await api.createAccount({
        name: newName.trim(),
        description: newDesc.trim() || undefined,
        broker: newBroker.trim() || undefined,
      })
      setNewName("")
      setNewDesc("")
      setNewBroker("")
      onRefresh()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "未知错误")
    }
  }

  const handleUpdate = async (id: string) => {
    if (!editName.trim()) return
    setError("")
    try {
      await api.updateAccount(id, {
        name: editName.trim(),
        description: editDesc.trim() || undefined,
        broker: editBroker.trim() || undefined,
      })
      setEditingId(null)
      onRefresh()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "未知错误")
    }
  }

  const confirmDelete = async () => {
    if (!deleteTarget) return
    setError("")
    try {
      await api.deleteAccount(deleteTarget.id)
      setDeleteTarget(null)
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
          <h2 className="text-lg font-semibold">管理账户</h2>
          <button
            onClick={onClose}
            className="text-[#6C757D] hover:text-[#1A1A1A] text-xl leading-none"
          >
            &times;
          </button>
        </div>

        {error && <div className="mb-3 p-2 bg-red-50 text-red-700 text-xs rounded">{error}</div>}

        <div className="mb-4 space-y-2">
          <input
            type="text"
            placeholder="账户名称"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            className="w-full text-sm border border-[#E9ECEF] rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
            onKeyDown={(e) => e.key === "Enter" && handleCreate()}
          />
          <input
            type="text"
            placeholder="券商/机构（可选）"
            value={newBroker}
            onChange={(e) => setNewBroker(e.target.value)}
            className="w-full text-sm border border-[#E9ECEF] rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
          />
          <input
            type="text"
            placeholder="描述（可选）"
            value={newDesc}
            onChange={(e) => setNewDesc(e.target.value)}
            className="w-full text-sm border border-[#E9ECEF] rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
          />
          <button
            onClick={handleCreate}
            disabled={!newName.trim()}
            className="w-full text-sm bg-[#1A1A1A] text-white px-3 py-1.5 rounded hover:bg-[#333] disabled:opacity-50 transition-colors"
          >
            创建账户
          </button>
        </div>

        <div className="space-y-2">
          {accounts.map((a) => (
            <div key={a.id} className="border border-[#E9ECEF] rounded p-3">
              {editingId === a.id ? (
                <div className="space-y-2">
                  <input
                    type="text"
                    value={editName}
                    onChange={(e) => setEditName(e.target.value)}
                    className="w-full text-sm border border-[#E9ECEF] rounded px-2 py-1 focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
                  />
                  <input
                    type="text"
                    value={editBroker}
                    onChange={(e) => setEditBroker(e.target.value)}
                    placeholder="券商/机构（可选）"
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
                      onClick={() => handleUpdate(a.id)}
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
                    <span className="text-sm font-medium">{a.name}</span>
                    {a.broker && (
                      <span className="ml-2 text-[10px] text-[#6C757D] bg-[#F8F9FA] px-1.5 py-0.5 rounded">
                        {a.broker}
                      </span>
                    )}
                    {a.description && (
                      <p className="text-xs text-[#6C757D] mt-0.5">{a.description}</p>
                    )}
                  </div>
                  <div className="flex gap-2">
                    <button
                      onClick={() => {
                        setEditingId(a.id)
                        setEditName(a.name)
                        setEditBroker(a.broker || "")
                        setEditDesc(a.description || "")
                      }}
                      className="text-xs text-[#6C757D] hover:text-[#1A1A1A]"
                    >
                      编辑
                    </button>
                    <button
                      onClick={() => setDeleteTarget({ id: a.id, name: a.name })}
                      className="text-xs text-red-500 hover:text-red-700"
                    >
                      删除
                    </button>
                  </div>
                </div>
              )}
            </div>
          ))}
          {accounts.length === 0 && (
            <p className="text-sm text-[#6C757D] text-center py-4">暂无账户，请创建</p>
          )}
        </div>
      </div>

      {deleteTarget && (
        <ConfirmDialog
          title="删除账户"
          message={`确定删除账户"${deleteTarget.name}"？该账户下的持仓将被解除归属，但持仓本身不会被删除。`}
          onConfirm={confirmDelete}
          onCancel={() => setDeleteTarget(null)}
        />
      )}
    </div>
  )
}
