import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import * as api from "../api"
import SetupWizard from "./SetupWizard"

vi.mock("../api", () => ({
  submitSetup: vi.fn(),
}))

describe("SetupWizard", () => {
  beforeEach(() => {
    vi.mocked(api.submitSetup).mockReset()
    vi.mocked(api.submitSetup).mockResolvedValue({ success: true })
  })

  it("submits human-readable database fields instead of a DSN", async () => {
    const onComplete = vi.fn()
    render(<SetupWizard onComplete={onComplete} />)

    fireEvent.click(screen.getByRole("button", { name: "下一步" }))
    expect(screen.queryByText(/DSN/)).not.toBeInTheDocument()

    fireEvent.change(screen.getByLabelText("主机地址"), { target: { value: "db.example.com" } })
    fireEvent.change(screen.getByLabelText("端口"), { target: { value: "5433" } })
    fireEvent.change(screen.getByLabelText("数据库名"), { target: { value: "investments" } })
    fireEvent.change(screen.getByLabelText("数据库用户"), { target: { value: "portfolio_app" } })
    fireEvent.change(screen.getByLabelText("数据库密码"), { target: { value: "db-password" } })
    fireEvent.change(screen.getByLabelText("SSL 连接"), { target: { value: "require" } })
    fireEvent.click(screen.getByRole("button", { name: "下一步" }))

    fireEvent.change(screen.getByPlaceholderText("用户名"), { target: { value: "admin" } })
    fireEvent.change(screen.getByPlaceholderText("密码（至少6位）"), {
      target: { value: "admin-password" },
    })
    fireEvent.click(screen.getByRole("button", { name: "完成配置" }))

    await waitFor(() => {
      expect(api.submitSetup).toHaveBeenCalledWith({
        databaseType: "postgres",
        databaseHost: "db.example.com",
        databasePort: "5433",
        databaseName: "investments",
        databaseUsername: "portfolio_app",
        databasePassword: "db-password",
        databaseSslMode: "require",
        username: "admin",
        password: "admin-password",
      })
    })
    expect(onComplete).toHaveBeenCalledOnce()
  })
})
