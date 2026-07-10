import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"
import type { Holding } from "../types"
import * as api from "../api"
import SellModal from "./SellModal"

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api")
  return {
    ...actual,
    sellHolding: vi.fn(),
  }
})

vi.mock("./toast-context", () => ({
  useToast: () => ({
    showToast: vi.fn(),
  }),
}))

const tinyHolding: Holding = {
  id: "holding-1",
  portfolioId: "portfolio-1",
  assetId: "stocks",
  symbol: "BTC",
  name: "Bitcoin",
  market: "CRYPTO",
  currency: "USD",
  shares: "0.00000001",
  price: "100000",
  value: "0.001",
  cost: "0.001",
  accountId: "account-1",
}

describe("SellModal", () => {
  it("submits tiny share strings without number coercion", async () => {
    vi.mocked(api.sellHolding).mockResolvedValue({
      soldHolding: tinyHolding,
      availableFunds: "0.001",
    })

    render(
      <SellModal
        portfolioId="portfolio-1"
        holding={tinyHolding}
        displayCurrency="USD"
        onConfirm={vi.fn()}
        onClose={vi.fn()}
      />
    )

    const inputs = screen.getAllByRole("spinbutton")
    fireEvent.change(inputs[0], { target: { value: "0.00000001" } })
    fireEvent.change(inputs[1], { target: { value: "100000" } })
    fireEvent.click(screen.getByRole("button", { name: "确认卖出" }))

    await waitFor(() => expect(api.sellHolding).toHaveBeenCalled())
    expect(api.sellHolding).toHaveBeenCalledWith(
      "portfolio-1",
      "holding-1",
      "0.00000001",
      "100000",
      "0",
      "0",
      expect.any(String)
    )
  })
})
