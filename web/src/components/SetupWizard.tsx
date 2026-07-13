import { useState } from "react"
import * as api from "../api"

interface SetupWizardProps {
  onComplete: () => void
}

export default function SetupWizard({ onComplete }: SetupWizardProps) {
  const [step, setStep] = useState(0)
  const [databaseType] = useState("postgres")
  const [databaseHost, setDatabaseHost] = useState("localhost")
  const [databasePort, setDatabasePort] = useState("5432")
  const [databaseName, setDatabaseName] = useState("portfolio")
  const [databaseUsername, setDatabaseUsername] = useState("postgres")
  const [databasePassword, setDatabasePassword] = useState("")
  const [databaseSslMode, setDatabaseSslMode] = useState("disable")
  const [adminUsername, setAdminUsername] = useState("")
  const [adminPassword, setAdminPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  const databasePortNumber = Number(databasePort)
  const isDatabaseConnectionValid =
    databaseHost.trim() !== "" &&
    databaseName.trim() !== "" &&
    Number.isInteger(databasePortNumber) &&
    databasePortNumber >= 1 &&
    databasePortNumber <= 65535

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
        databaseHost,
        databasePort,
        databaseName,
        databaseUsername,
        databasePassword,
        databaseSslMode,
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
      <div className="bg-white rounded-2xl shadow-xl w-full max-w-lg">
        <div className="px-6 pt-6 pb-4">
          <div className="flex items-center gap-3 mb-2">
            <img src="/logo.svg" alt="投资组合管理" className="w-8 h-8 shrink-0" />
            <h2 className="text-lg font-medium text-[#1A1A1A]">初始配置</h2>
          </div>
          <p className="text-sm text-[#6C757D]">请完成以下设置</p>
        </div>

        <div className="px-6 pb-6 space-y-6">
          {step === 0 && (
            <>
              <div>
                <label className="block text-sm font-medium text-[#1A1A1A] mb-2">数据库类型</label>
                <p className="text-xs text-[#6C757D] mb-3">选择用于存储数据的数据库类型。</p>
                <div className="flex gap-3">
                  <button className="flex-1 px-4 py-3 rounded-lg border text-sm transition-colors bg-[#1A1A1A] text-white border-[#1A1A1A] cursor-default">
                    <div className="font-medium">PostgreSQL</div>
                    <div className="text-xs mt-1 opacity-75">推荐，适合生产环境</div>
                  </button>
                </div>
              </div>

              <div className="flex justify-end pt-2">
                <button
                  onClick={() => setStep(1)}
                  className="px-4 py-2 text-sm bg-[#1A1A1A] text-white rounded-lg hover:bg-[#333] transition-colors"
                >
                  下一步
                </button>
              </div>
            </>
          )}

          {step === 1 && (
            <>
              <div>
                <label className="block text-sm font-medium text-[#1A1A1A] mb-2">数据库连接</label>
                <p className="text-xs text-[#6C757D] mb-4">
                  填写 PostgreSQL 的连接信息，系统会自动生成连接配置。
                </p>
                <div className="space-y-3">
                  <div className="grid grid-cols-[1fr_7rem] gap-3">
                    <div>
                      <label
                        htmlFor="database-host"
                        className="block text-xs font-medium text-[#6C757D] mb-1"
                      >
                        主机地址
                      </label>
                      <input
                        id="database-host"
                        type="text"
                        value={databaseHost}
                        onChange={(e) => setDatabaseHost(e.target.value)}
                        placeholder="localhost"
                        autoComplete="url"
                        className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                      />
                    </div>
                    <div>
                      <label
                        htmlFor="database-port"
                        className="block text-xs font-medium text-[#6C757D] mb-1"
                      >
                        端口
                      </label>
                      <input
                        id="database-port"
                        type="number"
                        min="1"
                        max="65535"
                        value={databasePort}
                        onChange={(e) => setDatabasePort(e.target.value)}
                        placeholder="5432"
                        className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                      />
                    </div>
                  </div>
                  <div>
                    <label
                      htmlFor="database-name"
                      className="block text-xs font-medium text-[#6C757D] mb-1"
                    >
                      数据库名
                    </label>
                    <input
                      id="database-name"
                      type="text"
                      value={databaseName}
                      onChange={(e) => setDatabaseName(e.target.value)}
                      placeholder="portfolio"
                      className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                    />
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label
                        htmlFor="database-username"
                        className="block text-xs font-medium text-[#6C757D] mb-1"
                      >
                        数据库用户
                      </label>
                      <input
                        id="database-username"
                        type="text"
                        value={databaseUsername}
                        onChange={(e) => setDatabaseUsername(e.target.value)}
                        placeholder="postgres"
                        autoComplete="username"
                        className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                      />
                    </div>
                    <div>
                      <label
                        htmlFor="database-password"
                        className="block text-xs font-medium text-[#6C757D] mb-1"
                      >
                        数据库密码
                      </label>
                      <input
                        id="database-password"
                        type="password"
                        value={databasePassword}
                        onChange={(e) => setDatabasePassword(e.target.value)}
                        placeholder="未设置可留空"
                        autoComplete="current-password"
                        className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                      />
                    </div>
                  </div>
                  <div>
                    <label
                      htmlFor="database-ssl-mode"
                      className="block text-xs font-medium text-[#6C757D] mb-1"
                    >
                      SSL 连接
                    </label>
                    <select
                      id="database-ssl-mode"
                      value={databaseSslMode}
                      onChange={(e) => setDatabaseSslMode(e.target.value)}
                      className="w-full px-3 py-2 text-sm bg-white border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                    >
                      <option value="disable">关闭（本机或可信内网）</option>
                      <option value="prefer">优先使用 SSL</option>
                      <option value="require">必须使用 SSL</option>
                      <option value="verify-full">SSL 并验证证书与主机名</option>
                    </select>
                  </div>
                  <p className="text-xs text-[#6C757D]">
                    本机数据库通常可使用默认值；云数据库请按服务商提供的信息填写。
                  </p>
                </div>
              </div>

              <div className="flex justify-between pt-2">
                <button
                  onClick={() => setStep(0)}
                  className="px-4 py-2 text-sm text-[#6C757D] hover:text-[#1A1A1A] transition-colors"
                >
                  上一步
                </button>
                <button
                  onClick={() => setStep(2)}
                  disabled={!isDatabaseConnectionValid}
                  className="px-4 py-2 text-sm bg-[#1A1A1A] text-white rounded-lg hover:bg-[#333] transition-colors disabled:opacity-50"
                >
                  下一步
                </button>
              </div>
            </>
          )}

          {step === 2 && (
            <>
              <div>
                <label className="block text-sm font-medium text-[#1A1A1A] mb-2">
                  创建管理员账户
                </label>
                <p className="text-xs text-[#6C757D] mb-3">管理员可以管理用户和系统设置。</p>
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

              {error && <p className="text-xs text-red-500">{error}</p>}

              <div className="flex justify-between pt-2">
                <button
                  onClick={() => setStep(1)}
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
