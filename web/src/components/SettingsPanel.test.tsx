import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import * as api from "../api"
import { DEFAULT_SETTINGS, Settings } from "../types"
import SettingsPanel from "./SettingsPanel"
import { ToastContext } from "./toast-context"

vi.mock("../api", () => ({
  fetchMarketSources: vi.fn(),
  updateMarketSources: vi.fn(),
  testMarketSourcesStream: vi.fn(),
  fetchOIDCConfig: vi.fn(),
  updateOIDCConfig: vi.fn(),
  fetchWebAuthnConfig: vi.fn(),
  updateWebAuthnConfig: vi.fn(),
  webAuthnListCredentials: vi.fn(),
  webAuthnRegisterStart: vi.fn(),
  webAuthnRegisterFinish: vi.fn(),
  webAuthnDeleteCredential: vi.fn(),
  testTelegramConnection: vi.fn(),
  testTelegramMessage: vi.fn(),
  testBarkConnection: vi.fn(),
  testBarkMessage: vi.fn(),
}))

const showToast = vi.fn()

function renderPanel({
  role = "user",
  settings = DEFAULT_SETTINGS,
  onSave = vi.fn().mockResolvedValue(undefined),
}: {
  role?: "admin" | "user"
  settings?: Settings
  onSave?: (settings: Settings) => void | Promise<void>
} = {}) {
  return {
    onSave,
    ...render(
      <ToastContext.Provider value={{ showToast }}>
        <SettingsPanel
          settings={settings}
          onSave={onSave}
          userRole={role}
          portfolioId="portfolio-1"
        />
      </ToastContext.Provider>
    ),
  }
}

describe("SettingsPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.fetchMarketSources).mockResolvedValue({
      available: {},
      config: {},
      sourceNames: {},
    })
    vi.mocked(api.updateMarketSources).mockResolvedValue({ status: "ok" })
    vi.mocked(api.webAuthnListCredentials).mockResolvedValue([])
    vi.mocked(api.fetchOIDCConfig).mockResolvedValue({
      enabled: false,
      issuer: "",
      clientID: "",
      clientSecret: "",
      redirectURL: "",
    })
    vi.mocked(api.updateOIDCConfig).mockImplementation(async (config) => config)
    vi.mocked(api.fetchWebAuthnConfig).mockResolvedValue({
      enabled: false,
      rpid: "",
      rpOrigins: [],
    })
    vi.mocked(api.updateWebAuthnConfig).mockImplementation(async (config) => config)
  })

  it("separates portfolio settings from user settings", async () => {
    renderPanel()
    fireEvent.click(screen.getByTitle("设置"))

    expect(screen.getByRole("tab", { name: "组合设置" })).toHaveAttribute("aria-selected", "true")
    expect(screen.getByText("再平衡漂移阈值")).toBeInTheDocument()
    expect(screen.queryByRole("tab", { name: "系统设置" })).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole("button", { name: "显示" }))
    expect(screen.getByText("涨跌配色")).toBeInTheDocument()
    expect(screen.getByText("显示币种")).toBeInTheDocument()

    fireEvent.click(screen.getByRole("tab", { name: "用户设置" }))
    await waitFor(() => expect(screen.getByText("行情源配置")).toBeInTheDocument())
    expect(screen.queryByText("涨跌配色")).not.toBeInTheDocument()
    expect(screen.queryByText("显示币种")).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole("button", { name: "安全" }))
    await waitFor(() => expect(screen.getByText("Passkey 管理")).toBeInTheDocument())
    expect(screen.queryByText("SSO 登录 (OIDC)")).not.toBeInTheDocument()
  })

  it("shows system settings only to administrators", async () => {
    renderPanel({ role: "admin" })
    fireEvent.click(screen.getByTitle("设置"))
    fireEvent.click(screen.getByRole("tab", { name: "系统设置" }))

    await waitFor(() => expect(screen.getByText("SSO 登录 (OIDC)")).toBeInTheDocument())
    expect(screen.getByText("Passkey 登录 (WebAuthn)")).toBeInTheDocument()
    expect(screen.queryByText("Passkey 管理")).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole("button", { name: "保存" }))
    await waitFor(() => expect(api.updateOIDCConfig).toHaveBeenCalledOnce())
    expect(api.updateWebAuthnConfig).toHaveBeenCalledOnce()
    expect(api.updateMarketSources).toHaveBeenCalledOnce()
  })

  it("preserves drafts across tabs and discards them after cancelling", async () => {
    const { onSave } = renderPanel()
    fireEvent.click(screen.getByTitle("设置"))
    fireEvent.click(screen.getByRole("button", { name: "显示" }))
    fireEvent.click(screen.getByRole("button", { name: "USD $" }))
    fireEvent.click(screen.getByRole("tab", { name: "用户设置" }))
    fireEvent.click(screen.getByRole("tab", { name: "组合设置" }))
    fireEvent.click(screen.getByRole("button", { name: "显示" }))
    fireEvent.click(screen.getByRole("button", { name: "保存" }))

    await waitFor(() => expect(showToast).toHaveBeenCalledWith("设置保存成功", "success"))
    expect(onSave).toHaveBeenCalledWith(expect.objectContaining({ displayCurrency: "USD" }))
    expect(api.updateMarketSources).toHaveBeenCalledOnce()

    fireEvent.click(screen.getByTitle("设置"))
    fireEvent.click(screen.getByRole("button", { name: "显示" }))
    fireEvent.click(screen.getByRole("button", { name: "EUR €" }))
    fireEvent.click(screen.getByRole("button", { name: "取消" }))
    fireEvent.click(screen.getByTitle("设置"))
    fireEvent.click(screen.getByRole("button", { name: "显示" }))
    fireEvent.click(screen.getByRole("button", { name: "保存" }))

    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(2))
    expect(onSave).toHaveBeenLastCalledWith(
      expect.objectContaining({ displayCurrency: DEFAULT_SETTINGS.displayCurrency })
    )
  })

  it("keeps the dialog and draft open when saving fails", async () => {
    const onSave = vi.fn().mockRejectedValue(new Error("network unavailable"))
    renderPanel({ onSave })
    fireEvent.click(screen.getByTitle("设置"))
    fireEvent.click(screen.getByRole("button", { name: "显示" }))
    fireEvent.click(screen.getByRole("button", { name: "USD $" }))
    fireEvent.click(screen.getByRole("button", { name: "保存" }))

    await waitFor(() =>
      expect(showToast).toHaveBeenCalledWith("设置保存失败：network unavailable", "error")
    )
    expect(screen.getByRole("button", { name: "保存" })).toBeInTheDocument()

    fireEvent.click(screen.getByRole("button", { name: "保存" }))
    await waitFor(() =>
      expect(onSave).toHaveBeenLastCalledWith(expect.objectContaining({ displayCurrency: "USD" }))
    )
  })
})
