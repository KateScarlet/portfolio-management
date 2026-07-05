import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"
import * as api from "../api"
import FundOperationDialog from "./FundOperationDialog"

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api")
  return {
    ...actual,
    convertCurrency: vi.fn(),
    fetchExchangeRate: vi.fn(),
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
})
