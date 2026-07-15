import { FormEvent, useCallback, useEffect, useState } from "react"
import {
  AlertCircle,
  Eye,
  EyeOff,
  LoaderCircle,
  ShieldCheck,
  Trash2,
  UserPlus,
  UserRound,
  Users,
  X,
} from "lucide-react"
import * as api from "../api"
import { UserInfo } from "../types"
import ConfirmDialog from "./ConfirmDialog"

interface Props {
  currentUser: UserInfo
}

export default function UserManager({ currentUser }: Props) {
  const [isOpen, setIsOpen] = useState(false)
  const [users, setUsers] = useState<UserInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [showAdd, setShowAdd] = useState(false)
  const [newUsername, setNewUsername] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [newRole, setNewRole] = useState<"admin" | "user">("user")
  const [showPassword, setShowPassword] = useState(false)
  const [formError, setFormError] = useState("")
  const [listError, setListError] = useState("")
  const [creating, setCreating] = useState(false)
  const [deletingUser, setDeletingUser] = useState<UserInfo | null>(null)
  const [deleting, setDeleting] = useState(false)

  const resetForm = useCallback(() => {
    setNewUsername("")
    setNewPassword("")
    setNewRole("user")
    setShowPassword(false)
    setFormError("")
  }, [])

  const closeDialog = useCallback(() => {
    if (creating || deleting) return
    setIsOpen(false)
    setShowAdd(false)
    resetForm()
  }, [creating, deleting, resetForm])

  useEffect(() => {
    if (!isOpen) return
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !deletingUser) closeDialog()
    }
    document.addEventListener("keydown", handleKeyDown)
    return () => document.removeEventListener("keydown", handleKeyDown)
  }, [isOpen, deletingUser, closeDialog])

  const loadUsers = async () => {
    setLoading(true)
    setListError("")
    try {
      setUsers(await api.listUsers())
    } catch (error) {
      setListError(error instanceof Error ? error.message : "用户列表加载失败")
    } finally {
      setLoading(false)
    }
  }

  const handleOpen = () => {
    setIsOpen(true)
    setShowAdd(false)
    resetForm()
    void loadUsers()
  }

  const handleAdd = async (event: FormEvent) => {
    event.preventDefault()
    const username = newUsername.trim()
    if (!username || !newPassword) {
      setFormError("请填写用户名和密码")
      return
    }
    if (newPassword.length < 6) {
      setFormError("密码至少需要 6 位")
      return
    }

    setCreating(true)
    setFormError("")
    try {
      const user = await api.register(username, newPassword, newRole)
      setUsers((current) => [...current, user])
      resetForm()
      setShowAdd(false)
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "创建用户失败")
    } finally {
      setCreating(false)
    }
  }

  const handleDelete = async () => {
    if (!deletingUser) return
    setDeleting(true)
    setListError("")
    try {
      await api.deleteUser(deletingUser.id)
      setUsers((current) => current.filter((user) => user.id !== deletingUser.id))
      setDeletingUser(null)
    } catch (error) {
      setListError(error instanceof Error ? error.message : "删除用户失败")
      setDeletingUser(null)
    } finally {
      setDeleting(false)
    }
  }

  const adminCount = users.filter((user) => user.role === "admin").length

  return (
    <>
      <button
        type="button"
        onClick={handleOpen}
        className="flex h-8 w-8 items-center justify-center rounded-lg text-[#6C757D] transition-colors hover:bg-[#F1F3F5] hover:text-[#1A1A1A]"
        title="用户管理"
        aria-label="用户管理"
      >
        <Users className="h-4 w-4" />
      </button>

      {isOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-[#1A1A1A]/45 p-4 backdrop-blur-[2px]"
          onMouseDown={(event) => event.target === event.currentTarget && closeDialog()}
        >
          <section
            role="dialog"
            aria-modal="true"
            aria-labelledby="user-manager-title"
            className="flex max-h-[min(760px,90vh)] w-full max-w-2xl flex-col overflow-hidden rounded-2xl bg-white shadow-2xl"
          >
            <header className="flex items-start justify-between border-b border-[#E9ECEF] px-5 py-5 sm:px-6">
              <div className="flex items-start gap-3">
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-[#F1F3F5] text-[#343A40]">
                  <Users className="h-5 w-5" />
                </div>
                <div>
                  <h2 id="user-manager-title" className="font-semibold text-[#1A1A1A]">
                    用户管理
                  </h2>
                  <p className="mt-0.5 text-xs leading-5 text-[#6C757D]">
                    邀请成员并管理他们的系统访问权限
                  </p>
                </div>
              </div>
              <button
                type="button"
                onClick={closeDialog}
                className="-mr-1 rounded-lg p-2 text-[#ADB5BD] transition-colors hover:bg-[#F1F3F5] hover:text-[#1A1A1A]"
                aria-label="关闭用户管理"
              >
                <X className="h-5 w-5" />
              </button>
            </header>

            <div className="flex min-h-0 flex-1 flex-col overflow-y-auto scrollbar-thin">
              <div className="flex flex-col gap-3 border-b border-[#F1F3F5] px-5 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-6">
                <div className="flex items-center gap-2 text-xs text-[#6C757D]">
                  <span className="rounded-full bg-[#F1F3F5] px-2.5 py-1">
                    {users.length} 位成员
                  </span>
                  <span>{adminCount} 位管理员</span>
                </div>
                <button
                  type="button"
                  onClick={() => {
                    setShowAdd((current) => !current)
                    setFormError("")
                  }}
                  className="inline-flex w-full items-center justify-center gap-2 rounded-lg bg-[#1A1A1A] px-3.5 py-2 text-sm font-medium text-white transition-colors hover:bg-[#343A40] sm:w-auto"
                >
                  {showAdd ? <X className="h-4 w-4" /> : <UserPlus className="h-4 w-4" />}
                  {showAdd ? "收起" : "添加用户"}
                </button>
              </div>

              {showAdd && (
                <form
                  onSubmit={handleAdd}
                  className="border-b border-[#E9ECEF] bg-[#F8F9FA] px-5 py-5 sm:px-6"
                >
                  <div className="mb-4">
                    <h3 className="text-sm font-semibold text-[#1A1A1A]">添加新用户</h3>
                    <p className="mt-1 text-xs text-[#6C757D]">
                      创建后，用户可立即使用该账号登录。
                    </p>
                  </div>
                  <div className="grid gap-4 sm:grid-cols-2">
                    <label className="block">
                      <span className="mb-1.5 block text-xs font-medium text-[#495057]">
                        用户名
                      </span>
                      <input
                        type="text"
                        value={newUsername}
                        onChange={(event) => setNewUsername(event.target.value)}
                        autoFocus
                        autoComplete="off"
                        placeholder="例如：wangming"
                        className="w-full rounded-lg border border-[#DEE2E6] bg-white px-3 py-2.5 text-sm text-[#1A1A1A] outline-none transition focus:border-[#868E96] focus:ring-2 focus:ring-[#1A1A1A]/10"
                      />
                    </label>
                    <label className="block">
                      <span className="mb-1.5 block text-xs font-medium text-[#495057]">角色</span>
                      <select
                        value={newRole}
                        onChange={(event) => setNewRole(event.target.value as "admin" | "user")}
                        className="w-full rounded-lg border border-[#DEE2E6] bg-white px-3 py-2.5 text-sm text-[#1A1A1A] outline-none transition focus:border-[#868E96] focus:ring-2 focus:ring-[#1A1A1A]/10"
                      >
                        <option value="user">普通用户</option>
                        <option value="admin">管理员</option>
                      </select>
                    </label>
                    <label className="block sm:col-span-2">
                      <span className="mb-1.5 block text-xs font-medium text-[#495057]">
                        初始密码
                      </span>
                      <div className="relative">
                        <input
                          type={showPassword ? "text" : "password"}
                          value={newPassword}
                          onChange={(event) => setNewPassword(event.target.value)}
                          autoComplete="new-password"
                          placeholder="至少 6 位字符"
                          className="w-full rounded-lg border border-[#DEE2E6] bg-white py-2.5 pl-3 pr-11 text-sm text-[#1A1A1A] outline-none transition focus:border-[#868E96] focus:ring-2 focus:ring-[#1A1A1A]/10"
                        />
                        <button
                          type="button"
                          onClick={() => setShowPassword((current) => !current)}
                          className="absolute inset-y-0 right-0 flex w-11 items-center justify-center text-[#868E96] hover:text-[#343A40]"
                          aria-label={showPassword ? "隐藏密码" : "显示密码"}
                        >
                          {showPassword ? (
                            <EyeOff className="h-4 w-4" />
                          ) : (
                            <Eye className="h-4 w-4" />
                          )}
                        </button>
                      </div>
                    </label>
                  </div>

                  {newRole === "admin" && (
                    <p className="mt-3 flex items-start gap-2 rounded-lg bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">
                      <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0" />
                      管理员可修改系统配置、添加用户并删除其他用户。
                    </p>
                  )}
                  {formError && (
                    <p role="alert" className="mt-3 flex items-center gap-2 text-xs text-red-600">
                      <AlertCircle className="h-4 w-4 shrink-0" />
                      {formError}
                    </p>
                  )}
                  <div className="mt-4 flex justify-end gap-2">
                    <button
                      type="button"
                      onClick={() => {
                        setShowAdd(false)
                        resetForm()
                      }}
                      className="rounded-lg px-3.5 py-2 text-sm text-[#6C757D] transition-colors hover:bg-[#E9ECEF] hover:text-[#1A1A1A]"
                    >
                      取消
                    </button>
                    <button
                      type="submit"
                      disabled={creating || !newUsername.trim() || !newPassword}
                      className="inline-flex min-w-24 items-center justify-center gap-2 rounded-lg bg-[#1A1A1A] px-3.5 py-2 text-sm font-medium text-white transition-colors hover:bg-[#343A40] disabled:cursor-not-allowed disabled:opacity-40"
                    >
                      {creating && <LoaderCircle className="h-4 w-4 animate-spin" />}
                      {creating ? "创建中" : "创建用户"}
                    </button>
                  </div>
                </form>
              )}

              <div className="px-5 py-5 sm:px-6">
                {listError && (
                  <div
                    role="alert"
                    className="mb-4 flex items-start justify-between gap-3 rounded-lg bg-red-50 px-3 py-2.5 text-xs text-red-700"
                  >
                    <span className="flex items-start gap-2">
                      <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
                      {listError}
                    </span>
                    <button
                      type="button"
                      onClick={() => void loadUsers()}
                      className="shrink-0 font-medium underline underline-offset-2"
                    >
                      重试
                    </button>
                  </div>
                )}

                {loading ? (
                  <div className="flex flex-col items-center justify-center py-12 text-[#868E96]">
                    <LoaderCircle className="mb-3 h-6 w-6 animate-spin" />
                    <p className="text-sm">正在加载用户…</p>
                  </div>
                ) : users.length === 0 && !listError ? (
                  <div className="flex flex-col items-center justify-center py-12 text-center">
                    <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-full bg-[#F1F3F5] text-[#868E96]">
                      <Users className="h-5 w-5" />
                    </div>
                    <p className="text-sm font-medium text-[#343A40]">暂无用户</p>
                    <p className="mt-1 text-xs text-[#868E96]">添加第一个用户以开始协作</p>
                  </div>
                ) : (
                  <ul className="divide-y divide-[#F1F3F5]">
                    {users.map((user) => {
                      const isCurrentUser = user.id === currentUser.id
                      return (
                        <li
                          key={user.id}
                          className="group flex items-center gap-3 py-3 first:pt-0 last:pb-0"
                        >
                          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-[#F1F3F5] text-[#6C757D]">
                            <UserRound className="h-4 w-4" />
                          </div>
                          <div className="min-w-0 flex-1">
                            <div className="flex min-w-0 items-center gap-2">
                              <span className="truncate text-sm font-medium text-[#1A1A1A]">
                                {user.username}
                              </span>
                              {isCurrentUser && (
                                <span className="shrink-0 text-[11px] text-[#868E96]">你</span>
                              )}
                            </div>
                            <p className="mt-0.5 text-xs text-[#868E96]">
                              {user.role === "admin"
                                ? "可管理系统设置与用户"
                                : "可访问自己的投资组合"}
                            </p>
                          </div>
                          <span
                            className={`shrink-0 rounded-full px-2.5 py-1 text-[11px] font-medium ${user.role === "admin" ? "bg-violet-50 text-violet-700" : "bg-[#F1F3F5] text-[#6C757D]"}`}
                          >
                            {user.role === "admin" ? "管理员" : "普通用户"}
                          </span>
                          <button
                            type="button"
                            onClick={() => setDeletingUser(user)}
                            disabled={isCurrentUser}
                            className="rounded-lg p-2 text-[#ADB5BD] transition-colors hover:bg-red-50 hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:bg-transparent disabled:hover:text-[#ADB5BD]"
                            title={isCurrentUser ? "不能删除当前登录用户" : `删除 ${user.username}`}
                            aria-label={
                              isCurrentUser ? "不能删除当前登录用户" : `删除 ${user.username}`
                            }
                          >
                            <Trash2 className="h-4 w-4" />
                          </button>
                        </li>
                      )
                    })}
                  </ul>
                )}
              </div>
            </div>
          </section>
        </div>
      )}

      {deletingUser && (
        <ConfirmDialog
          title="删除用户"
          message={`确定删除“${deletingUser.username}”吗？该用户的投资组合及相关数据也会被永久删除，此操作不可撤销。`}
          onConfirm={() => void handleDelete()}
          onCancel={() => !deleting && setDeletingUser(null)}
        />
      )}
    </>
  )
}
