import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import * as api from "../api"
import { Account } from "../types"
import AccountManager from "./AccountManager"

vi.mock("../api", () => ({
  createAccount: vi.fn(),
  updateAccount: vi.fn(),
  deleteAccount: vi.fn(),
}))

const accounts: Account[] = [
  {
    id: "account-default",
    userId: "user-1",
    name: "默认账户",
    description: "资产归集",
    isDefault: true,
    createdAt: "2026-01-01T00:00:00Z",
  },
  {
    id: "account-broker",
    userId: "user-1",
    name: "股票账户",
    broker: "华泰证券",
    isDefault: false,
    createdAt: "2026-02-01T00:00:00Z",
  },
]

describe("AccountManager", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("explains and protects the default account", () => {
    render(<AccountManager accounts={accounts} onClose={vi.fn()} onRefresh={vi.fn()} />)

    expect(screen.getByText("2 个账户")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "默认账户不能删除" })).toBeDisabled()
    expect(screen.getByRole("button", { name: "删除 股票账户" })).toBeEnabled()
    expect(screen.getByText("华泰证券")).toBeInTheDocument()
  })

  it("creates a trimmed account and refreshes the list", async () => {
    const onRefresh = vi.fn().mockResolvedValue(undefined)
    vi.mocked(api.createAccount).mockResolvedValue(accounts[1])
    render(<AccountManager accounts={accounts} onClose={vi.fn()} onRefresh={onRefresh} />)

    fireEvent.click(screen.getByRole("button", { name: "新建账户" }))
    fireEvent.change(screen.getByLabelText("账户名称"), {
      target: { value: "  港股账户  " },
    })
    fireEvent.change(screen.getByLabelText(/券商 \/ 机构/), {
      target: { value: "  富途证券  " },
    })
    fireEvent.change(screen.getByLabelText(/描述/), { target: { value: "  长期持有  " } })
    fireEvent.click(screen.getByRole("button", { name: "创建账户" }))

    await waitFor(() =>
      expect(api.createAccount).toHaveBeenCalledWith({
        name: "港股账户",
        broker: "富途证券",
        description: "长期持有",
      })
    )
    expect(onRefresh).toHaveBeenCalledOnce()
    await waitFor(() => expect(screen.queryByText("创建资产账户")).not.toBeInTheDocument())
  })

  it("edits an account and explains how holdings are handled before deletion", async () => {
    const onRefresh = vi.fn().mockResolvedValue(undefined)
    vi.mocked(api.updateAccount).mockResolvedValue({
      ...accounts[1],
      name: "海外账户",
      broker: "",
    })
    vi.mocked(api.deleteAccount).mockResolvedValue(undefined)
    render(<AccountManager accounts={accounts} onClose={vi.fn()} onRefresh={onRefresh} />)

    fireEvent.click(screen.getByRole("button", { name: "编辑 股票账户" }))
    fireEvent.change(screen.getByLabelText("账户名称"), { target: { value: "海外账户" } })
    fireEvent.change(screen.getByLabelText("券商 / 机构"), { target: { value: "" } })
    fireEvent.click(screen.getByRole("button", { name: "保存" }))

    await waitFor(() =>
      expect(api.updateAccount).toHaveBeenCalledWith("account-broker", {
        name: "海外账户",
        broker: "",
        description: "",
      })
    )

    fireEvent.click(screen.getByRole("button", { name: "删除 股票账户" }))
    expect(screen.getByText(/持仓不会丢失，将自动迁移到默认账户/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole("button", { name: "确认" }))

    await waitFor(() => expect(api.deleteAccount).toHaveBeenCalledWith("account-broker"))
    expect(onRefresh).toHaveBeenCalledTimes(2)
  })
})
