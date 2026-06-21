import { useState } from "react"
import * as api from "../api"

interface SetupWizardProps {
  onComplete: () => void
}

export default function SetupWizard({ onComplete }: SetupWizardProps) {
  const [step, setStep] = useState(0)
  const [databaseType, setDatabaseType] = useState("sqlite")
  const [databaseDsn, setDatabaseDsn] = useState("")
  const [adminUsername, setAdminUsername] = useState("")
  const [adminPassword, setAdminPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  const handleSubmit = async () => {
    if (!adminUsername || !adminPassword) {
      setError("请填写管理员用户名和密码")
      return
    }
    if (adminPassword.length < 6) {
      setError("密码至少6位")
      return
    }

    setLoading(true)
    setError("")
    try {
      await api.submitSetup({
        databaseType,
        databaseDsn,
        username: adminUsername,
        password: adminPassword,
      })
      onComplete()
    } catch (e) {
      setError(e instanceof Error ? e.message : "配置保存失败")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-[#F8F9FA] flex items-center justify-center p-4">
      <div className="bg-white rounded-2xl shadow-xl w-full max-w-md">
        <div className="px-6 pt-6 pb-4">
          <div className="flex items-center gap-3 mb-2">
            <div className="w-8 h-8 bg-[#1A1A1A] rounded-md flex items-center justify-center">
              <div className="w-4 h-4 border-2 border-white rounded-full"></div>
            </div>
            <h2 className="text-lg font-medium text-[#1A1A1A]">初始配置</h2>
          </div>
          <p className="text-sm text-[#6C757D]">请完成以下设置</p>
        </div>

        <div className="px-6 pb-6 space-y-6">
          {step === 0 && (
            <>
              <div>
                <label className="block text-sm font-medium text-[#1A1A1A] mb-2">
                  数据库类型
                </label>
                <p className="text-xs text-[#6C757D] mb-3">
                  选择用于存储数据的数据库类型。
                </p>
                <div className="flex gap-3">
                  <button
                    onClick={() => setDatabaseType("sqlite")}
                    className={`flex-1 px-4 py-3 rounded-lg border text-sm transition-colors ${
                      databaseType === "sqlite"
                        ? "bg-[#1A1A1A] text-white border-[#1A1A1A]"
                        : "bg-white text-[#6C757D] border-[#E9ECEF] hover:border-[#ADB5BD]"
                    }`}
                  >
                    <div className="font-medium">SQLite</div>
                    <div className="text-xs mt-1 opacity-75">零配置，适合个人使用</div>
                  </button>
                  <button
                    onClick={() => setDatabaseType("postgres")}
                    className={`flex-1 px-4 py-3 rounded-lg border text-sm transition-colors ${
                      databaseType === "postgres"
                        ? "bg-[#1A1A1A] text-white border-[#1A1A1A]"
                        : "bg-white text-[#6C757D] border-[#E9ECEF] hover:border-[#ADB5BD]"
                    }`}
                  >
                    <div className="font-medium">PostgreSQL</div>
                    <div className="text-xs mt-1 opacity-75">适合多用户部署</div>
                  </button>
                </div>
              </div>

              {databaseType === "postgres" && (
                <div>
                  <label className="block text-xs font-medium text-[#6C757D] mb-1">
                    连接地址 (DSN)
                  </label>
                  <input
                    type="text"
                    value={databaseDsn}
                    onChange={(e) => setDatabaseDsn(e.target.value)}
                    placeholder="postgres://user:pass@localhost:5432/portfolio?sslmode=disable"
                    className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                  />
                  <p className="text-xs text-[#6C757D] mt-1">
                    格式: postgres://用户名:密码@主机:端口/数据库名?sslmode=disable
                  </p>
                </div>
              )}

              <div className="flex justify-end pt-2">
                <button
                  onClick={() => setStep(1)}
                  disabled={databaseType === "postgres" && !databaseDsn}
                  className="px-4 py-2 text-sm bg-[#1A1A1A] text-white rounded-lg hover:bg-[#333] transition-colors disabled:opacity-50"
                >
                  下一步
                </button>
              </div>
            </>
          )}

          {step === 1 && (
            <>
              <div>
                <label className="block text-sm font-medium text-[#1A1A1A] mb-2">
                  创建管理员账户
                </label>
                <p className="text-xs text-[#6C757D] mb-3">
                  管理员可以管理用户和系统设置。
                </p>
                <div className="space-y-3">
                  <input
                    type="text"
                    placeholder="用户名"
                    value={adminUsername}
                    onChange={(e) => setAdminUsername(e.target.value)}
                    className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                  />
                  <input
                    type="password"
                    placeholder="密码（至少6位）"
                    value={adminPassword}
                    onChange={(e) => setAdminPassword(e.target.value)}
                    className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                  />
                </div>
              </div>

              {error && (
                <p className="text-xs text-red-500">{error}</p>
              )}

              <div className="flex justify-between pt-2">
                <button
                  onClick={() => setStep(0)}
                  className="px-4 py-2 text-sm text-[#6C757D] hover:text-[#1A1A1A] transition-colors"
                >
                  上一步
                </button>
                <button
                  onClick={handleSubmit}
                  disabled={loading || !adminUsername || !adminPassword}
                  className="px-4 py-2 text-sm bg-[#1A1A1A] text-white rounded-lg hover:bg-[#333] transition-colors disabled:opacity-50"
                >
                  {loading ? "保存中..." : "完成配置"}
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
