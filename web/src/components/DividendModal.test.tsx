import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import type { Dividend, Holding } from "../types"
import * as api from "../api"
import DividendModal from "./DividendModal"

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api")
  return { ...actual, recordDividend: vi.fn(), updateDividend: vi.fn() }
})

const showToast = vi.fn()
vi.mock("./toast-context", () => ({ useToast: () => ({ showToast }) }))

const holding: Holding = {
  id: "holding-1",
  portfolioId: "portfolio-1",
  accountId: "account-1",
  assetId: "stocks",
  symbol: "AAPL",
  name: "Apple",
  currency: "USD",
  shares: "10",
  price: "20",
  value: "200",
}

function dividendResult(): Dividend {
  return {
    id: "dividend-1",
    userId: "user-1",
    portfolioId: "portfolio-1",
    holdingId: holding.id,
    type: "reinvest",
    grossAmount: "110",
    taxAmount: "10",
    netAmount: "100",
    currency: "USD",
    paymentDate: "2026-07-13T00:00:00Z",
    sharesAtPayment: "10",
    reinvestmentPrice: "20",
    reinvestedShares: "5",
    fundTxId: "fund-tx-1",
    createdAt: "2026-07-13T00:00:00Z",
    updatedAt: "2026-07-13T00:00:00Z",
  }
}

describe("DividendModal", () => {
  beforeEach(() => vi.clearAllMocks())

  it("submits a reinvestment using the holding currency", async () => {
    vi.mocked(api.recordDividend).mockResolvedValue(dividendResult())
    render(
      <DividendModal
        portfolioId="portfolio-1"
        holding={holding}
        onConfirm={vi.fn()}
        onClose={vi.fn()}
      />
    )

    fireEvent.change(screen.getByLabelText("分红总额"), { target: { value: "110" } })
    fireEvent.change(screen.getByLabelText("预扣税"), { target: { value: "10" } })
    fireEvent.click(screen.getByRole("button", { name: "红利再投资" }))
    fireEvent.change(screen.getByLabelText("再投资价格"), { target: { value: "20" } })
    fireEvent.click(screen.getByRole("button", { name: "确认" }))

    await waitFor(() => expect(api.recordDividend).toHaveBeenCalled())
    expect(api.recordDividend).toHaveBeenCalledWith(
      "portfolio-1",
      expect.objectContaining({
        holdingId: "holding-1",
        grossAmount: "110",
        taxAmount: "10",
        type: "reinvest",
        reinvestmentPrice: "20",
      })
    )
    expect(api.recordDividend).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ currency: expect.anything() })
    )
  })

  it("rejects an empty amount without throwing", () => {
    render(
      <DividendModal
        portfolioId="portfolio-1"
        holding={holding}
        onConfirm={vi.fn()}
        onClose={vi.fn()}
      />
    )
    fireEvent.click(screen.getByRole("button", { name: "确认" }))
    expect(showToast).toHaveBeenCalledWith("请输入大于 0 的分红总额", "error")
    expect(api.recordDividend).not.toHaveBeenCalled()
  })
})
