import { useState, useEffect } from "react"
import * as api from "../api"

interface LoginPageProps {
  onLogin: () => void
}

export default function LoginPage({ onLogin }: LoginPageProps) {
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const [oidcEnabled, setOidcEnabled] = useState(false)
  const [webauthnEnabled, setWebauthnEnabled] = useState(false)

  useEffect(() => {
    fetch("/api/auth/oidc/status")
      .then((res) => res.json())
      .then((data) => setOidcEnabled(data.enabled))
      .catch(() => {})
    fetch("/api/webauthn/status")
      .then((res) => res.json())
      .then((data) => setWebauthnEnabled(data.enabled))
      .catch(() => {})
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!username || !password) {
      setError("请输入用户名和密码")
      return
    }

    setLoading(true)
    setError("")
    try {
      await api.login(username, password)
      onLogin()
    } catch (e) {
      setError(e instanceof Error ? e.message : "登录失败")
    } finally {
      setLoading(false)
    }
  }

  const handlePasskeyLogin = async () => {
    setLoading(true)
    setError("")
    try {
      const options = await api.webAuthnLoginStart()
      const { startAuthentication } = await import("@simplewebauthn/browser")
      const credential = await startAuthentication({ optionsJSON: options })
      await api.webAuthnLoginFinish(credential)
      onLogin()
    } catch (e) {
      setError(e instanceof Error ? e.message : "Passkey登录失败")
    } finally {
      setLoading(false)
    }
  }

  const showDivider = oidcEnabled || webauthnEnabled

  return (
    <div className="min-h-screen bg-[#F8F9FA] flex items-center justify-center p-4">
      <div className="bg-white rounded-2xl shadow-xl w-full max-w-sm">
        <div className="px-6 pt-6 pb-4">
          <div className="flex items-center gap-3 mb-4">
            <div className="w-8 h-8 bg-[#1A1A1A] rounded-md flex items-center justify-center">
              <div className="w-4 h-4 border-2 border-white rounded-full"></div>
            </div>
            <h1 className="text-lg font-semibold text-[#1A1A1A]">投资组合管理</h1>
          </div>
          <p className="text-sm text-[#6C757D]">请登录以继续</p>
        </div>

        <form onSubmit={handleSubmit} className="px-6 pb-6 space-y-4">
          <div>
            <label className="block text-xs font-medium text-[#6C757D] mb-1">
              用户名
            </label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoFocus
              className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-[#6C757D] mb-1">
              密码
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
            />
          </div>

          {error && (
            <p className="text-xs text-red-500">{error}</p>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full px-4 py-2 text-sm bg-[#1A1A1A] text-white rounded-lg hover:bg-[#333] transition-colors disabled:opacity-50"
          >
            {loading ? "登录中..." : "登录"}
          </button>

          {showDivider && (
            <>
              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t border-[#E9ECEF]"></div>
                </div>
                <div className="relative flex justify-center text-xs">
                  <span className="bg-white px-2 text-[#6C757D]">或</span>
                </div>
              </div>
            </>
          )}

          {webauthnEnabled && window.PublicKeyCredential !== undefined && (
            <button
              type="button"
              onClick={handlePasskeyLogin}
              disabled={loading}
              className="w-full px-4 py-2 text-sm border border-[#E9ECEF] text-[#1A1A1A] rounded-lg hover:bg-[#F8F9FA] transition-colors disabled:opacity-50"
            >
              Passkey 登录
            </button>
          )}

          {oidcEnabled && (
            <a
              href="/api/auth/oidc"
              className="w-full px-4 py-2 text-sm border border-[#E9ECEF] text-[#1A1A1A] rounded-lg hover:bg-[#F8F9FA] transition-colors text-center block"
            >
              SSO 登录
            </a>
          )}
        </form>
      </div>
    </div>
  )
}
