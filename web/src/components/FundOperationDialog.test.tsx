import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { describe, expect, it, vi } from "vite-plus/test"
import * as api from "../api"
import FundOperationDialog from "./FundOperationDialog"

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api")
  return {
    ...actual,
    convertCurrency: vi.fn(),
    fetchExchangeRate: vi.fn(),
    transferOutFunds: vi.fn(),
  }
})

vi.mock("./toast-context", () => ({
  useToast: () => ({
    showToast: vi.fn(),
  }),
}))

describe("FundOperationDialog", () => {
  it("keeps tiny conversion amounts and long rates as decimal strings", async () => {
    vi.mocked(api.convertCurrency).mockResolvedValue({ status: "ok" })

    render(
      <FundOperationDialog
        type="convert"
        portfolios={[]}
        availableFunds={[{ currency: "USD", amount: "100" }]}
        currentPortfolioId="portfolio-1"
        currentCurrency="USD"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    )

    const inputs = screen.getAllByRole("spinbutton")
    fireEvent.change(inputs[0], { target: { value: "0.00000001" } })
    fireEvent.change(inputs[2], { target: { value: "7.123456789123456789" } })

    await waitFor(() => expect(inputs[1]).toHaveValue(0.00000007123456789123457))
    fireEvent.click(screen.getByRole("button", { name: "确认" }))

    await waitFor(() => expect(api.convertCurrency).toHaveBeenCalled())
    expect(api.convertCurrency).toHaveBeenCalledWith(
      "portfolio-1",
      "USD",
      "CNY",
      "0.00000001",
      "7.123456789123456789e-8",
      "7.123456789123456789"
    )
  })

  it("only offers currencies with available funds for transfers out", () => {
    render(
      <FundOperationDialog
        type="transfer_out"
        portfolios={[]}
        availableFunds={[
          { currency: "USD", amount: "100" },
          { currency: "HKD", amount: "200" },
        ]}
        currentPortfolioId="portfolio-1"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    )

    const currencySelect = screen.getByRole("combobox")
    expect(currencySelect).toHaveValue("USD")
    expect(screen.getByRole("option", { name: "USD" })).toBeInTheDocument()
    expect(screen.getByRole("option", { name: "HKD" })).toBeInTheDocument()
    expect(screen.queryByRole("option", { name: "CNY" })).not.toBeInTheDocument()
    expect(screen.queryByRole("option", { name: "EUR" })).not.toBeInTheDocument()
  })

  it("shows the available balance and supports filling the full amount", () => {
    render(
      <FundOperationDialog
        type="transfer_out"
        portfolios={[]}
        availableFunds={[{ currency: "USD", amount: "123.45" }]}
        currentPortfolioId="portfolio-1"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    )

    expect(screen.getByRole("dialog", { name: "转出资金" })).toBeInTheDocument()
    expect(screen.getByText(/可用余额/)).toHaveTextContent("$123.45")

    fireEvent.click(screen.getByRole("button", { name: "全部" }))
    expect(screen.getByRole("spinbutton", { name: "操作金额" })).toHaveValue(123.45)
  })

  it("prevents an amount greater than the available balance", async () => {
    render(
      <FundOperationDialog
        type="transfer_out"
        portfolios={[]}
        availableFunds={[{ currency: "USD", amount: "100" }]}
        currentPortfolioId="portfolio-1"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    )

    fireEvent.change(screen.getByRole("spinbutton", { name: "操作金额" }), {
      target: { value: "100.01" },
    })
    fireEvent.click(screen.getByRole("button", { name: "确认" }))

    expect(await screen.findByRole("alert")).toHaveTextContent("金额不能超过可用余额")
    expect(api.transferOutFunds).not.toHaveBeenCalled()
  })

  it("disables transfers out when there are no available fund currencies", () => {
    render(
      <FundOperationDialog
        type="transfer_out"
        portfolios={[]}
        availableFunds={[]}
        currentPortfolioId="portfolio-1"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    )

    expect(screen.getByRole("option", { name: "暂无可用币种" })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "确认" })).toBeDisabled()
  })

  it("only offers currencies with available funds for transfers between portfolios", () => {
    render(
      <FundOperationDialog
        type="transfer"
        portfolios={[
          {
            id: "portfolio-1",
            userId: "user-1",
            name: "当前组合",
            isDefault: true,
            createdAt: "2026-07-15T00:00:00Z",
          },
          {
            id: "portfolio-2",
            userId: "user-1",
            name: "目标组合",
            isDefault: false,
            createdAt: "2026-07-15T00:00:00Z",
          },
        ]}
        availableFunds={[{ currency: "EUR", amount: "300" }]}
        currentPortfolioId="portfolio-1"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    )

    const selects = screen.getAllByRole("combobox")
    expect(selects[0]).toHaveValue("EUR")
    expect(screen.getByRole("option", { name: "EUR" })).toBeInTheDocument()
    expect(screen.queryByRole("option", { name: "CNY" })).not.toBeInTheDocument()
  })

  it("only offers available fund currencies as conversion sources", () => {
    render(
      <FundOperationDialog
        type="convert"
        portfolios={[]}
        availableFunds={[{ currency: "HKD", amount: "200" }]}
        currentPortfolioId="portfolio-1"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    )

    const selects = screen.getAllByRole("combobox")
    expect(selects[0]).toHaveValue("HKD")
    expect(selects[0].querySelectorAll("option")).toHaveLength(1)
    expect(selects[1]).toHaveValue("CNY")
    expect(selects[1].querySelector('option[value="USD"]')).toBeInTheDocument()
    expect(screen.getByTitle("目标币种没有可用资金，无法互换")).toBeDisabled()
  })
})
