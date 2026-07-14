import { FormEvent, useCallback, useEffect, useState } from "react"
import {
  AlertCircle,
  Check,
  LoaderCircle,
  Pencil,
  PieChart,
  Plus,
  Save,
  Star,
  Trash2,
  X,
} from "lucide-react"
import * as api from "../api"
import { Portfolio } from "../types"
import ConfirmDialog from "./ConfirmDialog"

interface Props {
  portfolios: Portfolio[]
  onClose: () => void
  onRefresh: () => void | Promise<void>
}

type PendingAction = "create" | "delete" | string | null

export default function PortfolioManager({ portfolios, onClose, onRefresh }: Props) {
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState("")
  const [newDesc, setNewDesc] = useState("")
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editName, setEditName] = useState("")
  const [editDesc, setEditDesc] = useState("")
  const [error, setError] = useState("")
  const [pendingAction, setPendingAction] = useState<PendingAction>(null)
  const [deleteTarget, setDeleteTarget] = useState<Portfolio | null>(null)

  const isBusy = pendingAction !== null

  const resetCreateForm = useCallback(() => {
    setNewName("")
    setNewDesc("")
    setError("")
  }, [])

  const closeDialog = useCallback(() => {
    if (isBusy || deleteTarget) return
    onClose()
  }, [deleteTarget, isBusy, onClose])

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        if (isBusy || deleteTarget) return
        if (editingId) {
          setEditingId(null)
          setError("")
        } else if (showCreate) {
          setShowCreate(false)
          resetCreateForm()
        } else {
          closeDialog()
        }
      }
    }
    document.addEventListener("keydown", handleKeyDown)
    return () => document.removeEventListener("keydown", handleKeyDown)
  }, [closeDialog, deleteTarget, editingId, isBusy, resetCreateForm, showCreate])

  const handleCreate = async (event: FormEvent) => {
    event.preventDefault()
    const name = newName.trim()
    if (!name) {
      setError("请输入投资组合名称")
      return
    }

    setPendingAction("create")
    setError("")
    try {
      await api.createPortfolio(name, newDesc.trim() || undefined)
      await onRefresh()
      resetCreateForm()
      setShowCreate(false)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "创建投资组合失败")
    } finally {
      setPendingAction(null)
    }
  }

  const startEditing = (portfolio: Portfolio) => {
    setShowCreate(false)
    resetCreateForm()
    setEditingId(portfolio.id)
    setEditName(portfolio.name)
    setEditDesc(portfolio.description || "")
  }

  const handleUpdate = async (event: FormEvent, id: string) => {
    event.preventDefault()
    const name = editName.trim()
    if (!name) {
      setError("投资组合名称不能为空")
      return
    }

    setPendingAction(id)
    setError("")
    try {
      await api.updatePortfolio(id, {
        name,
        description: editDesc.trim() || undefined,
      })
      await onRefresh()
      setEditingId(null)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "保存投资组合失败")
    } finally {
      setPendingAction(null)
    }
  }

  const confirmDelete = async () => {
    if (!deleteTarget || isBusy) return
    setPendingAction("delete")
    setError("")
    try {
      await api.deletePortfolio(deleteTarget.id)
      await onRefresh()
      setDeleteTarget(null)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "删除投资组合失败")
      setDeleteTarget(null)
    } finally {
      setPendingAction(null)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-[#1A1A1A]/45 p-4 backdrop-blur-[2px]"
      onMouseDown={(event) => event.target === event.currentTarget && closeDialog()}
    >
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby="portfolio-manager-title"
        className="flex max-h-[min(780px,90vh)] w-full max-w-2xl flex-col overflow-hidden rounded-2xl bg-white shadow-2xl"
      >
        <header className="flex items-start justify-between border-b border-[#E9ECEF] px-5 py-5 sm:px-6">
          <div className="flex items-start gap-3">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-[#F1F3F5] text-[#343A40]">
              <PieChart className="h-5 w-5" />
            </div>
            <div>
              <h2 id="portfolio-manager-title" className="font-semibold text-[#1A1A1A]">
                管理投资组合
              </h2>
              <p className="mt-0.5 text-xs leading-5 text-[#6C757D]">
                创建并整理不同投资目标下的资产
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={closeDialog}
            disabled={isBusy}
            className="-mr-1 rounded-lg p-2 text-[#ADB5BD] transition-colors hover:bg-[#F1F3F5] hover:text-[#1A1A1A] disabled:cursor-not-allowed disabled:opacity-40"
            aria-label="关闭投资组合管理"
          >
            <X className="h-5 w-5" />
          </button>
        </header>

        <div className="flex min-h-0 flex-1 flex-col overflow-y-auto scrollbar-thin">
          <div className="flex flex-col gap-3 border-b border-[#F1F3F5] px-5 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-6">
            <div>
              <p className="text-sm font-medium text-[#343A40]">{portfolios.length} 个投资组合</p>
              <p className="mt-0.5 text-xs text-[#868E96]">默认组合用于承接你的初始数据</p>
            </div>
            <button
              type="button"
              onClick={() => {
                setShowCreate((current) => !current)
                setEditingId(null)
                resetCreateForm()
              }}
              disabled={isBusy}
              className="inline-flex w-full items-center justify-center gap-2 rounded-lg bg-[#1A1A1A] px-3.5 py-2 text-sm font-medium text-white transition-colors hover:bg-[#343A40] disabled:cursor-not-allowed disabled:opacity-40 sm:w-auto"
            >
              {showCreate ? <X className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
              {showCreate ? "收起" : "新建组合"}
            </button>
          </div>

          {showCreate && (
            <form
              onSubmit={handleCreate}
              className="border-b border-[#E9ECEF] bg-[#F8F9FA] px-5 py-5 sm:px-6"
            >
              <div className="mb-4">
                <h3 className="text-sm font-semibold text-[#1A1A1A]">创建投资组合</h3>
                <p className="mt-1 text-xs text-[#6C757D]">
                  可按投资目标、风险策略或家庭成员划分资产。
                </p>
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <label className="block">
                  <span className="mb-1.5 block text-xs font-medium text-[#495057]">组合名称</span>
                  <input
                    type="text"
                    value={newName}
                    onChange={(event) => setNewName(event.target.value)}
                    autoFocus
                    placeholder="例如：长期养老"
                    className="w-full rounded-lg border border-[#DEE2E6] bg-white px-3 py-2.5 text-sm text-[#1A1A1A] outline-none transition focus:border-[#868E96] focus:ring-2 focus:ring-[#1A1A1A]/10"
                  />
                </label>
                <label className="block">
                  <span className="mb-1.5 block text-xs font-medium text-[#495057]">
                    描述 <span className="font-normal text-[#ADB5BD]">（可选）</span>
                  </span>
                  <input
                    type="text"
                    value={newDesc}
                    onChange={(event) => setNewDesc(event.target.value)}
                    placeholder="记录目标或策略"
                    className="w-full rounded-lg border border-[#DEE2E6] bg-white px-3 py-2.5 text-sm text-[#1A1A1A] outline-none transition focus:border-[#868E96] focus:ring-2 focus:ring-[#1A1A1A]/10"
                  />
                </label>
              </div>
              {error && (
                <p role="alert" className="mt-3 flex items-center gap-2 text-xs text-red-600">
                  <AlertCircle className="h-4 w-4 shrink-0" />
                  {error}
                </p>
              )}
              <div className="mt-4 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => {
                    setShowCreate(false)
                    resetCreateForm()
                  }}
                  disabled={isBusy}
                  className="rounded-lg px-3.5 py-2 text-sm text-[#6C757D] transition-colors hover:bg-[#E9ECEF] hover:text-[#1A1A1A] disabled:opacity-40"
                >
                  取消
                </button>
                <button
                  type="submit"
                  disabled={isBusy || !newName.trim()}
                  className="inline-flex min-w-24 items-center justify-center gap-2 rounded-lg bg-[#1A1A1A] px-3.5 py-2 text-sm font-medium text-white transition-colors hover:bg-[#343A40] disabled:cursor-not-allowed disabled:opacity-40"
                >
                  {pendingAction === "create" && <LoaderCircle className="h-4 w-4 animate-spin" />}
                  {pendingAction === "create" ? "创建中" : "创建组合"}
                </button>
              </div>
            </form>
          )}

          <div className="px-5 py-5 sm:px-6">
            {error && !showCreate && !editingId && (
              <div
                role="alert"
                className="mb-4 flex items-center gap-2 rounded-lg bg-red-50 px-3 py-2.5 text-xs text-red-700"
              >
                <AlertCircle className="h-4 w-4 shrink-0" />
                {error}
              </div>
            )}

            {portfolios.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-center">
                <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-full bg-[#F1F3F5] text-[#868E96]">
                  <PieChart className="h-5 w-5" />
                </div>
                <p className="text-sm font-medium text-[#343A40]">暂无投资组合</p>
                <p className="mt-1 text-xs text-[#868E96]">创建一个组合来开始记录资产</p>
              </div>
            ) : (
              <ul className="space-y-3">
                {portfolios.map((portfolio) => {
                  const isEditing = editingId === portfolio.id
                  const isSaving = pendingAction === portfolio.id
                  return (
                    <li
                      key={portfolio.id}
                      className={`rounded-xl border transition-colors ${
                        isEditing
                          ? "border-[#ADB5BD] bg-[#F8F9FA]"
                          : "border-[#E9ECEF] bg-white hover:border-[#CED4DA]"
                      }`}
                    >
                      {isEditing ? (
                        <form
                          onSubmit={(event) => void handleUpdate(event, portfolio.id)}
                          className="p-4"
                        >
                          <div className="mb-4 flex items-center justify-between">
                            <p className="flex items-center gap-2 text-sm font-semibold text-[#343A40]">
                              <Pencil className="h-4 w-4" /> 编辑组合
                            </p>
                            {portfolio.isDefault && (
                              <span className="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2.5 py-1 text-[11px] font-medium text-amber-700">
                                <Star className="h-3 w-3" /> 默认组合
                              </span>
                            )}
                          </div>
                          <div className="grid gap-4 sm:grid-cols-2">
                            <label className="block">
                              <span className="mb-1.5 block text-xs font-medium text-[#495057]">
                                组合名称
                              </span>
                              <input
                                type="text"
                                value={editName}
                                onChange={(event) => setEditName(event.target.value)}
                                autoFocus
                                className="w-full rounded-lg border border-[#DEE2E6] bg-white px-3 py-2.5 text-sm outline-none transition focus:border-[#868E96] focus:ring-2 focus:ring-[#1A1A1A]/10"
                              />
                            </label>
                            <label className="block">
                              <span className="mb-1.5 block text-xs font-medium text-[#495057]">
                                描述 <span className="font-normal text-[#ADB5BD]">（可选）</span>
                              </span>
                              <input
                                type="text"
                                value={editDesc}
                                onChange={(event) => setEditDesc(event.target.value)}
                                placeholder="记录目标或策略"
                                className="w-full rounded-lg border border-[#DEE2E6] bg-white px-3 py-2.5 text-sm outline-none transition focus:border-[#868E96] focus:ring-2 focus:ring-[#1A1A1A]/10"
                              />
                            </label>
                          </div>
                          {error && (
                            <p
                              role="alert"
                              className="mt-3 flex items-center gap-2 text-xs text-red-600"
                            >
                              <AlertCircle className="h-4 w-4 shrink-0" />
                              {error}
                            </p>
                          )}
                          <div className="mt-4 flex justify-end gap-2">
                            <button
                              type="button"
                              onClick={() => {
                                setEditingId(null)
                                setError("")
                              }}
                              disabled={isBusy}
                              className="rounded-lg px-3.5 py-2 text-sm text-[#6C757D] transition-colors hover:bg-[#E9ECEF] hover:text-[#1A1A1A] disabled:opacity-40"
                            >
                              取消
                            </button>
                            <button
                              type="submit"
                              disabled={isBusy || !editName.trim()}
                              className="inline-flex min-w-20 items-center justify-center gap-2 rounded-lg bg-[#1A1A1A] px-3.5 py-2 text-sm font-medium text-white transition-colors hover:bg-[#343A40] disabled:cursor-not-allowed disabled:opacity-40"
                            >
                              {isSaving ? (
                                <LoaderCircle className="h-4 w-4 animate-spin" />
                              ) : (
                                <Save className="h-4 w-4" />
                              )}
                              {isSaving ? "保存中" : "保存"}
                            </button>
                          </div>
                        </form>
                      ) : (
                        <div className="flex items-center gap-3 p-4">
                          <div
                            className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-xl ${
                              portfolio.isDefault
                                ? "bg-amber-50 text-amber-700"
                                : "bg-[#F1F3F5] text-[#6C757D]"
                            }`}
                          >
                            {portfolio.isDefault ? (
                              <Star className="h-4 w-4" />
                            ) : (
                              <PieChart className="h-4 w-4" />
                            )}
                          </div>
                          <div className="min-w-0 flex-1">
                            <div className="flex min-w-0 items-center gap-2">
                              <span className="truncate text-sm font-semibold text-[#1A1A1A]">
                                {portfolio.name}
                              </span>
                              {portfolio.isDefault && (
                                <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-amber-50 px-2 py-0.5 text-[10px] font-medium text-amber-700">
                                  <Check className="h-3 w-3" /> 默认
                                </span>
                              )}
                            </div>
                            <p className="mt-1 truncate text-xs text-[#868E96]">
                              {portfolio.description ||
                                (portfolio.isDefault
                                  ? "系统默认投资组合，不可删除"
                                  : "尚未添加组合描述")}
                            </p>
                          </div>
                          <div className="flex shrink-0 items-center gap-1">
                            <button
                              type="button"
                              onClick={() => startEditing(portfolio)}
                              disabled={isBusy}
                              className="rounded-lg p-2 text-[#868E96] transition-colors hover:bg-[#F1F3F5] hover:text-[#343A40] disabled:opacity-40"
                              title={`编辑 ${portfolio.name}`}
                              aria-label={`编辑 ${portfolio.name}`}
                            >
                              <Pencil className="h-4 w-4" />
                            </button>
                            <button
                              type="button"
                              onClick={() => {
                                setError("")
                                setDeleteTarget(portfolio)
                              }}
                              disabled={portfolio.isDefault || isBusy}
                              className="rounded-lg p-2 text-[#ADB5BD] transition-colors hover:bg-red-50 hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:bg-transparent disabled:hover:text-[#ADB5BD]"
                              title={
                                portfolio.isDefault ? "默认组合不能删除" : `删除 ${portfolio.name}`
                              }
                              aria-label={
                                portfolio.isDefault ? "默认组合不能删除" : `删除 ${portfolio.name}`
                              }
                            >
                              <Trash2 className="h-4 w-4" />
                            </button>
                          </div>
                        </div>
                      )}
                    </li>
                  )
                })}
              </ul>
            )}
          </div>
        </div>
      </section>

      {deleteTarget && (
        <ConfirmDialog
          title="删除投资组合"
          message={`确定删除“${deleteTarget.name}”吗？其中的持仓、资金流水、分红和历史记录都会被永久删除，此操作不可撤销。`}
          onConfirm={() => void confirmDelete()}
          onCancel={() => !isBusy && setDeleteTarget(null)}
        />
      )}
    </div>
  )
}
