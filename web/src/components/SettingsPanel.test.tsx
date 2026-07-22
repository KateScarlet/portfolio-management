import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vite-plus/test"
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

  it.each([
    ["股票", 0, -25, "targetStocks", "targetBonds"],
    ["长期债券", 0, 25, "targetBonds", "targetStocks"],
    ["现金", 2, -25, "targetCash", "targetCommodities"],
    ["商品", 2, 25, "targetCommodities", "targetCash"],
  ] as const)(
    "allows %s target allocation to be dragged to zero and saved",
    async (assetName, handleIndex, deltaX, zeroKey, adjacentKey) => {
      const { container, onSave } = renderPanel()
      fireEvent.click(screen.getByTitle("设置"))

      const handles = container.querySelectorAll<HTMLElement>(".cursor-col-resize")
      const handle = handles[handleIndex]
      const bar = handle.parentElement?.parentElement
      expect(handle).toBeDefined()
      expect(bar).not.toBeNull()

      Object.defineProperty(handle, "setPointerCapture", { value: vi.fn() })
      vi.spyOn(bar as HTMLElement, "getBoundingClientRect").mockReturnValue({
        width: 100,
        height: 40,
        top: 0,
        right: 100,
        bottom: 40,
        left: 0,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      })

      fireEvent.pointerDown(handle, { clientX: 50, pointerId: 1 })
      fireEvent.pointerMove(window, { clientX: 50 + deltaX, pointerId: 1 })
      fireEvent.pointerUp(window, { pointerId: 1 })

      expect(screen.getByText(`${assetName} 0%`)).toBeInTheDocument()
      fireEvent.click(screen.getByRole("button", { name: "保存" }))

      await waitFor(() => expect(onSave).toHaveBeenCalledOnce())
      const saved = vi.mocked(onSave).mock.calls[0][0]
      expect(saved[zeroKey]).toBe(0)
      expect(saved[adjacentKey]).toBe(50)
      expect(
        saved.targetStocks + saved.targetBonds + saved.targetCash + saved.targetCommodities
      ).toBe(100)
      expect(
        [saved.targetStocks, saved.targetBonds, saved.targetCash, saved.targetCommodities].every(
          (value) => value >= 0 && value <= 100
        )
      ).toBe(true)
    }
  )

  it("allows recovering from 100% allocation by dragging back", async () => {
    const { container, onSave } = renderPanel({
      settings: {
        ...DEFAULT_SETTINGS,
        targetStocks: 100,
        targetBonds: 0,
        targetCash: 0,
        targetCommodities: 0,
      },
    })
    fireEvent.click(screen.getByTitle("设置"))

    const handles = container.querySelectorAll<HTMLElement>(".cursor-col-resize")
    const handle0 = handles[0]
    const bar = handle0.parentElement?.parentElement
    expect(handle0).toBeDefined()
    expect(bar).not.toBeNull()

    Object.defineProperty(handle0, "setPointerCapture", { value: vi.fn() })
    vi.spyOn(bar as HTMLElement, "getBoundingClientRect").mockReturnValue({
      width: 200,
      height: 40,
      top: 0,
      right: 200,
      bottom: 40,
      left: 0,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    })

    // Drag handle0 left by 100px to reduce stocks from 100% by 50%
    fireEvent.pointerDown(handle0, { clientX: 150, pointerId: 1 })
    fireEvent.pointerMove(window, { clientX: 50, pointerId: 1 })
    fireEvent.pointerUp(window, { pointerId: 1 })

    fireEvent.click(screen.getByRole("button", { name: "保存" }))
    await waitFor(() => expect(onSave).toHaveBeenCalledOnce())
    const saved = vi.mocked(onSave).mock.calls[0][0]
    expect(saved.targetStocks).toBeLessThan(100)
    expect(saved.targetStocks).toBeGreaterThan(0)
    expect(saved.targetBonds).toBeGreaterThan(0)
    expect(saved.targetStocks + saved.targetBonds).toBe(100)
  })
})
