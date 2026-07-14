import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import * as api from "../api"
import UserManager from "./UserManager"

vi.mock("../api", () => ({
  listUsers: vi.fn(),
  register: vi.fn(),
  deleteUser: vi.fn(),
}))

const currentUser = { id: "admin-1", username: "owner", role: "admin" as const }
const member = { id: "user-1", username: "member", role: "user" as const }

describe("UserManager", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.listUsers).mockResolvedValue([currentUser, member])
  })

  it("identifies the current user and prevents deleting it", async () => {
    render(<UserManager currentUser={currentUser} />)
    fireEvent.click(screen.getByRole("button", { name: "用户管理" }))

    await waitFor(() => expect(screen.getByText("2 位成员")).toBeInTheDocument())
    expect(screen.getByText("你")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "不能删除当前登录用户" })).toBeDisabled()
    expect(screen.getByRole("button", { name: "删除 member" })).toBeEnabled()
  })

  it("creates a trimmed user and updates the member list", async () => {
    vi.mocked(api.register).mockResolvedValue({
      id: "user-2",
      username: "new-member",
      role: "user",
    })
    render(<UserManager currentUser={currentUser} />)
    fireEvent.click(screen.getByRole("button", { name: "用户管理" }))
    await screen.findByText("2 位成员")

    fireEvent.click(screen.getByRole("button", { name: "添加用户" }))
    fireEvent.change(screen.getByLabelText("用户名"), { target: { value: "  new-member  " } })
    fireEvent.change(screen.getByLabelText("初始密码"), { target: { value: "secret12" } })
    fireEvent.click(screen.getByRole("button", { name: "创建用户" }))

    await waitFor(() => expect(api.register).toHaveBeenCalledWith("new-member", "secret12", "user"))
    expect(await screen.findByText("3 位成员")).toBeInTheDocument()
    expect(screen.getByText("new-member")).toBeInTheDocument()
  })

  it("shows load failures and lets the administrator retry", async () => {
    vi.mocked(api.listUsers)
      .mockRejectedValueOnce(new Error("网络不可用"))
      .mockResolvedValueOnce([currentUser])
    render(<UserManager currentUser={currentUser} />)
    fireEvent.click(screen.getByRole("button", { name: "用户管理" }))

    expect(await screen.findByText("网络不可用")).toBeInTheDocument()
    fireEvent.click(screen.getByRole("button", { name: "重试" }))
    expect(await screen.findByText("1 位成员")).toBeInTheDocument()
    expect(api.listUsers).toHaveBeenCalledTimes(2)
  })
})
