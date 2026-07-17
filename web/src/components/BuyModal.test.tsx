import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { describe, expect, it, vi } from "vite-plus/test"
import type { Holding } from "../types"
import * as api from "../api"
import BuyModal from "./BuyModal"

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api")
  return {
    ...actual,
    createHolding: vi.fn(),
  }
})

vi.mock("./toast-context", () => ({
  useToast: () => ({
    showToast: vi.fn(),
  }),
}))

const baseHolding: Holding = {
  id: "holding-1",
  portfolioId: "portfolio-1",
  assetId: "stocks",
  symbol: "AAPL",
  name: "Apple",
  market: "US",
  currency: "USD",
  shares: "1",
  price: "10",
  value: "10",
  accountId: "account-1",
}

describe("BuyModal", () => {
  it("allows zero buy price for symbol-based holdings", async () => {
    vi.mocked(api.createHolding).mockResolvedValue(baseHolding)

    render(
      <BuyModal
        portfolioId="portfolio-1"
        holding={baseHolding}
        onConfirm={vi.fn()}
        onClose={vi.fn()}
      />
    )

    const inputs = screen.getAllByRole("spinbutton")
    fireEvent.change(inputs[0], { target: { value: "2" } })
    fireEvent.change(inputs[1], { target: { value: "0" } })
    fireEvent.click(screen.getByRole("button", { name: "确认买入" }))

    await waitFor(() => expect(api.createHolding).toHaveBeenCalled())
    expect(api.createHolding).toHaveBeenCalledWith(
      "portfolio-1",
      expect.objectContaining({
        costPrice: "0",
        cost: "0",
      })
    )
  })
})
