import { render } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import AssetIcon from "./AssetIcon"
import type { AssetId } from "../types"

describe("AssetIcon", () => {
  const assetIds: AssetId[] = ["stocks", "bonds", "cash", "commodities"]

  it("renders all asset types without errors", () => {
    for (const id of assetIds) {
      const { container, unmount } = render(<AssetIcon assetId={id} />)
      const div = container.firstElementChild as HTMLElement
      expect(div).toBeInTheDocument()
      expect(div.tagName).toBe("DIV")
      unmount()
    }
  })

  it("cash has border style", () => {
    const { container } = render(<AssetIcon assetId="cash" />)
    const div = container.firstElementChild as HTMLElement
    expect(div.className).toContain("border")
  })

  it("non-cash assets have no border", () => {
    const { container } = render(<AssetIcon assetId="stocks" />)
    const div = container.firstElementChild as HTMLElement
    expect(div.className).not.toContain("border")
  })
})
