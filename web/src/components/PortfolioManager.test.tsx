import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import * as api from "../api"
import { Portfolio } from "../types"
import PortfolioManager from "./PortfolioManager"

vi.mock("../api", () => ({
  createPortfolio: vi.fn(),
  updatePortfolio: vi.fn(),
  deletePortfolio: vi.fn(),
}))

const portfolios: Portfolio[] = [
  {
    id: "portfolio-default",
    userId: "user-1",
    name: "默认组合",
    description: "长期配置",
    isDefault: true,
    createdAt: "2026-01-01T00:00:00Z",
  },
  {
    id: "portfolio-growth",
    userId: "user-1",
    name: "成长组合",
    isDefault: false,
    createdAt: "2026-02-01T00:00:00Z",
  },
]

describe("PortfolioManager", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("explains and protects the default portfolio", () => {
    render(<PortfolioManager portfolios={portfolios} onClose={vi.fn()} onRefresh={vi.fn()} />)

    expect(screen.getByText("2 个投资组合")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "默认组合不能删除" })).toBeDisabled()
    expect(screen.getByRole("button", { name: "删除 成长组合" })).toBeEnabled()
  })

  it("creates a trimmed portfolio and refreshes the list", async () => {
    const onRefresh = vi.fn().mockResolvedValue(undefined)
    vi.mocked(api.createPortfolio).mockResolvedValue(portfolios[1])
    render(<PortfolioManager portfolios={portfolios} onClose={vi.fn()} onRefresh={onRefresh} />)

    fireEvent.click(screen.getByRole("button", { name: "新建组合" }))
    fireEvent.change(screen.getByLabelText("组合名称"), {
      target: { value: "  稳健养老  " },
    })
    fireEvent.change(screen.getByLabelText(/描述/), { target: { value: "  十年计划  " } })
    fireEvent.click(screen.getByRole("button", { name: "创建组合" }))

    await waitFor(() => expect(api.createPortfolio).toHaveBeenCalledWith("稳健养老", "十年计划"))
    expect(onRefresh).toHaveBeenCalledOnce()
    await waitFor(() => expect(screen.queryByText("创建投资组合")).not.toBeInTheDocument())
  })

  it("edits and deletes a non-default portfolio with explicit confirmation", async () => {
    const onRefresh = vi.fn().mockResolvedValue(undefined)
    vi.mocked(api.updatePortfolio).mockResolvedValue({
      ...portfolios[1],
      name: "全球成长",
    })
    vi.mocked(api.deletePortfolio).mockResolvedValue(undefined)
    render(<PortfolioManager portfolios={portfolios} onClose={vi.fn()} onRefresh={onRefresh} />)

    fireEvent.click(screen.getByRole("button", { name: "编辑 成长组合" }))
    fireEvent.change(screen.getByLabelText("组合名称"), { target: { value: "全球成长" } })
    fireEvent.click(screen.getByRole("button", { name: "保存" }))
    await waitFor(() =>
      expect(api.updatePortfolio).toHaveBeenCalledWith("portfolio-growth", {
        name: "全球成长",
        description: undefined,
      })
    )

    fireEvent.click(screen.getByRole("button", { name: "删除 成长组合" }))
    expect(screen.getByText(/资金流水、分红和历史记录/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole("button", { name: "确认" }))

    await waitFor(() => expect(api.deletePortfolio).toHaveBeenCalledWith("portfolio-growth"))
    expect(onRefresh).toHaveBeenCalledTimes(2)
  })
})
