import { useState } from "react"
import { UserInfo } from "../types"
import { Users, Trash2, UserPlus } from "lucide-react"
import * as api from "../api"
import ConfirmDialog from "./ConfirmDialog"

export default function UserManager() {
  const [isOpen, setIsOpen] = useState(false)
  const [users, setUsers] = useState<UserInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [showAdd, setShowAdd] = useState(false)
  const [newUsername, setNewUsername] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [newRole, setNewRole] = useState<"admin" | "user">("user")
  const [error, setError] = useState("")
  const [deletingUserId, setDeletingUserId] = useState<string | null>(null)

  const loadUsers = async () => {
    setLoading(true)
    try {
      const data = await api.listUsers()
      setUsers(data)
    } catch (e) {
      console.error("Failed to load users", e)
    } finally {
      setLoading(false)
    }
  }

  const handleOpen = async () => {
    setIsOpen(true)
    setShowAdd(false)
    setError("")
    await loadUsers()
  }

  const handleAdd = async () => {
    if (!newUsername || !newPassword) {
      setError("请填写用户名和密码")
      return
    }
    if (newPassword.length < 6) {
      setError("密码至少6位")
      return
    }

    try {
      const user = await api.register(newUsername, newPassword, newRole)
      setUsers([...users, user])
      setNewUsername("")
      setNewPassword("")
      setNewRole("user")
      setShowAdd(false)
      setError("")
    } catch (e) {
      setError(e instanceof Error ? e.message : "创建失败")
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await api.deleteUser(id)
      setUsers(users.filter((u) => u.id !== id))
    } catch (e) {
      console.error("Failed to delete user", e)
    }
    setDeletingUserId(null)
  }

  return (
    <>
      <button
        onClick={handleOpen}
        className="p-2 rounded-lg hover:bg-[#F1F3F5] transition-colors text-[#6C757D] hover:text-[#1A1A1A]"
        title="用户管理"
      >
        <Users className="w-5 h-5" />
      </button>

      {isOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/20"
          onClick={() => setIsOpen(false)}
        >
          <div
            className="bg-white rounded-2xl shadow-xl w-full max-w-sm mx-4 max-h-[70vh] flex flex-col"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between px-6 pt-6 pb-4">
              <h3 className="text-lg font-medium text-[#1A1A1A]">用户管理</h3>
              <button
                onClick={() => setIsOpen(false)}
                className="text-[#ADB5BD] hover:text-[#1A1A1A] text-xl leading-none"
              >
                &times;
              </button>
            </div>

            <div className="px-6 pb-2 space-y-4 overflow-y-auto flex-1 min-h-0">
              <div className="flex justify-end">
                <button
                  onClick={() => setShowAdd(!showAdd)}
                  className="flex items-center gap-1 px-3 py-1.5 text-xs text-[#1A1A1A] border border-[#E9ECEF] rounded-lg hover:bg-[#F1F3F5] transition-colors"
                >
                  <UserPlus className="w-3.5 h-3.5" />
                  添加用户
                </button>
              </div>

              {showAdd && (
                <div className="p-4 bg-[#F8F9FA] rounded-lg space-y-3">
                  <input
                    type="text"
                    placeholder="用户名"
                    value={newUsername}
                    onChange={(e) => setNewUsername(e.target.value)}
                    className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                  />
                  <input
                    type="password"
                    placeholder="密码（至少6位）"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                  />
                  <select
                    value={newRole}
                    onChange={(e) => setNewRole(e.target.value as "admin" | "user")}
                    className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                  >
                    <option value="user">普通用户</option>
                    <option value="admin">管理员</option>
                  </select>
                  {error && <p className="text-xs text-red-500">{error}</p>}
                  <div className="flex gap-2">
                    <button
                      onClick={handleAdd}
                      className="px-4 py-2 text-sm bg-[#1A1A1A] text-white rounded-lg hover:bg-[#333] transition-colors"
                    >
                      创建
                    </button>
                    <button
                      onClick={() => {
                        setShowAdd(false)
                        setError("")
                      }}
                      className="px-4 py-2 text-sm text-[#6C757D] hover:text-[#1A1A1A] transition-colors"
                    >
                      取消
                    </button>
                  </div>
                </div>
              )}

              {loading ? (
                <p className="text-sm text-[#6C757D] py-4">加载中...</p>
              ) : (
                <div className="space-y-2">
                  {users.map((user) => (
                    <div
                      key={user.id}
                      className="flex items-center justify-between p-3 bg-[#F8F9FA] rounded-lg"
                    >
                      <div>
                        <span className="text-sm text-[#1A1A1A]">{user.username}</span>
                        <span className="ml-2 text-xs text-[#6C757D]">
                          {user.role === "admin" ? "管理员" : "用户"}
                        </span>
                      </div>
                      <button
                        onClick={() => setDeletingUserId(user.id)}
                        className="p-1.5 text-[#6C757D] hover:text-red-500 transition-colors"
                        title="删除用户"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {deletingUserId && (
        <ConfirmDialog
          title="删除用户"
          message="确定删除此用户？此操作不可撤销。"
          onConfirm={() => handleDelete(deletingUserId)}
          onCancel={() => setDeletingUserId(null)}
        />
      )}
    </>
  )
}
