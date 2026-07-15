import { fireEvent, render, screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"
import type { PortfolioSummary } from "../types"
import SummaryDashboard from "./SummaryDashboard"

const summary: PortfolioSummary = {
  total: "150000",
  principal: "120000",
  assets: {
    stocks: "75000",
    bonds: "30000",
    cash: "30000",
    commodities: "15000",
  },
  portfolios: [
    {
      id: "smaller",
      name: "稳健组合",
      total: "50000",
      principal: "55000",
      assets: { stocks: "10000", bonds: "20000", cash: "15000", commodities: "5000" },
    },
    {
      id: "larger",
      name: "长期增长",
      total: "100000",
      principal: "65000",
      assets: { stocks: "65000", bonds: "10000", cash: "15000", commodities: "10000" },
    },
  ],
}

describe("SummaryDashboard", () => {
  it("provides loading and retry feedback", () => {
    const { rerender } = render(
      <SummaryDashboard
        summary={null}
        loading
        colorScheme="green-up"
        displayCurrency="CNY"
        onClose={vi.fn()}
      />
    )
    expect(screen.getByText("正在汇总全部投资组合")).toBeInTheDocument()

    const onRetry = vi.fn()
    rerender(
      <SummaryDashboard
        summary={null}
        error="服务暂时不可用"
        onRetry={onRetry}
        colorScheme="green-up"
        displayCurrency="CNY"
        onClose={vi.fn()}
      />
    )
    expect(screen.getByText("服务暂时不可用")).toBeInTheDocument()
    fireEvent.click(screen.getByRole("button", { name: "重新加载" }))
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it("shows aggregate performance, allocation, and portfolios ordered by value", () => {
    render(
      <SummaryDashboard
        summary={summary}
        colorScheme="green-up"
        displayCurrency="CNY"
        onClose={vi.fn()}
      />
    )

    expect(screen.getByRole("dialog", { name: "投资组合汇总" })).toBeInTheDocument()
    expect(screen.getByText("+25.00%")).toHaveClass("text-emerald-600")
    expect(screen.getByText("50.0%")).toBeInTheDocument()

    const portfolioNames = screen
      .getAllByRole("heading", { level: 4 })
      .map((node) => node.textContent)
    expect(portfolioNames).toEqual(["长期增长", "稳健组合"])
  })

  it("closes with Escape and restores page scrolling on unmount", () => {
    const onClose = vi.fn()
    const { unmount } = render(
      <SummaryDashboard
        summary={summary}
        colorScheme="red-up"
        displayCurrency="CNY"
        onClose={onClose}
      />
    )

    expect(document.body.style.overflow).toBe("hidden")
    fireEvent.keyDown(document, { key: "Escape" })
    expect(onClose).toHaveBeenCalledOnce()

    unmount()
    expect(document.body.style.overflow).toBe("")
  })
})
