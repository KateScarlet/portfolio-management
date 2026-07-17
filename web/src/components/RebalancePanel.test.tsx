import { render, screen } from "@testing-library/react"
import { describe, it, expect, vi } from "vite-plus/test"
import RebalancePanel from "./RebalancePanel"

vi.mock("./AssetIcon", () => ({
  default: ({ assetId }: { assetId: string }) => <div data-testid={`icon-${assetId}`} />,
}))

const defaultTargetPcts = {
  stocks: 25,
  bonds: 25,
  cash: 25,
  commodities: 25,
}

describe("RebalancePanel", () => {
  it("returns null when total is zero", () => {
    const { container } = render(
      <RebalancePanel
        assets={{ stocks: "0", bonds: "0", cash: "0", commodities: "0" }}
        total="0"
        driftThreshold={5}
        colorScheme="red-up"
        targetPcts={defaultTargetPcts}
        displayCurrency="CNY"
      />
    )
    expect(container.innerHTML).toBe("")
  })

  it("shows healthy message when all assets are within threshold", () => {
    render(
      <RebalancePanel
        assets={{ stocks: "2500", bonds: "2500", cash: "2500", commodities: "2500" }}
        total="10000"
        driftThreshold={5}
        colorScheme="red-up"
        targetPcts={defaultTargetPcts}
        displayCurrency="CNY"
      />
    )
    expect(screen.getByText("资产比例健康，无需调整")).toBeInTheDocument()
  })

  it("shows '补仓' when asset exceeds target upward", () => {
    render(
      <RebalancePanel
        assets={{ stocks: "3500", bonds: "2500", cash: "2500", commodities: "1500" }}
        total="10000"
        driftThreshold={5}
        colorScheme="red-up"
        targetPcts={defaultTargetPcts}
        displayCurrency="CNY"
      />
    )
    expect(screen.getByText("补仓")).toBeInTheDocument()
  })

  it("shows '减仓' when asset exceeds target downward", () => {
    render(
      <RebalancePanel
        assets={{ stocks: "1500", bonds: "2500", cash: "3000", commodities: "3000" }}
        total="10000"
        driftThreshold={5}
        colorScheme="red-up"
        targetPcts={defaultTargetPcts}
        displayCurrency="CNY"
      />
    )
    const sellButtons = screen.getAllByText("减仓")
    expect(sellButtons.length).toBeGreaterThan(0)
  })

  it("shows '保持' when drift is within threshold", () => {
    render(
      <RebalancePanel
        assets={{ stocks: "2400", bonds: "2500", cash: "2600", commodities: "2500" }}
        total="10000"
        driftThreshold={5}
        colorScheme="red-up"
        targetPcts={defaultTargetPcts}
        displayCurrency="CNY"
      />
    )
    const keepButtons = screen.getAllByText("保持")
    expect(keepButtons.length).toBeGreaterThan(0)
  })

  it("shows '补仓' when drift is just above threshold", () => {
    render(
      <RebalancePanel
        assets={{ stocks: "3100", bonds: "2500", cash: "2500", commodities: "1900" }}
        total="10000"
        driftThreshold={5}
        colorScheme="red-up"
        targetPcts={defaultTargetPcts}
        displayCurrency="CNY"
      />
    )
    expect(screen.getByText("补仓")).toBeInTheDocument()
  })
})
