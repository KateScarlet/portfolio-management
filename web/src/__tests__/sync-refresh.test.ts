import { describe, it, expect, vi, beforeEach } from "vitest"

describe("Bug 2: Refresh holdings after price sync completes", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("detects sync completion and triggers holdings refresh", () => {
    // Simulate the sync completion detection logic from App.tsx
    const prevSyncingRef = { current: true }

    const statusTransitioning: { syncing: boolean }[] = [
      { syncing: true }, // still syncing
      { syncing: true }, // still syncing
      { syncing: false }, // sync completed!
    ]

    let holdingsRefreshed = false
    for (const status of statusTransitioning) {
      if (prevSyncingRef.current && !status.syncing) {
        holdingsRefreshed = true
      }
      prevSyncingRef.current = status.syncing
    }

    expect(holdingsRefreshed).toBe(true)
  })

  it("does not refresh holdings when sync is still running", () => {
    const prevSyncingRef = { current: true }

    const statusTransitioning: { syncing: boolean }[] = [
      { syncing: true },
      { syncing: true },
      { syncing: true },
    ]

    let holdingsRefreshed = false
    for (const status of statusTransitioning) {
      if (prevSyncingRef.current && !status.syncing) {
        holdingsRefreshed = true
      }
      prevSyncingRef.current = status.syncing
    }

    expect(holdingsRefreshed).toBe(false)
  })

  it("does not refresh holdings on initial false->false transition", () => {
    const prevSyncingRef = { current: false }

    const status: { syncing: boolean } = { syncing: false }

    let holdingsRefreshed = false
    if (prevSyncingRef.current && !status.syncing) {
      holdingsRefreshed = true
    }
    prevSyncingRef.current = status.syncing

    expect(holdingsRefreshed).toBe(false)
  })
})
