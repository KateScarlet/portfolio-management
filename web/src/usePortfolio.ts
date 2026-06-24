import { useState, useEffect, useCallback } from "react"
import { AssetId, Holding, PortfolioRecord } from "./types"
import * as api from "./api"

export function usePortfolio(portfolioId: string | null, displayCurrency: string = "CNY") {
  const [holdings, setHoldings] = useState<Holding[]>([])
  const [history, setHistory] = useState<PortfolioRecord[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    if (!portfolioId) {
      return
    }
    const fetch = async () => {
      setLoading(true)
      try {
        const [h, r] = await Promise.all([
          api.fetchHoldings(portfolioId, displayCurrency),
          api.fetchRecords(portfolioId),
        ])
        if (!cancelled) {
          setHoldings(h)
          setHistory(r)
        }
      } catch (e) {
        console.error(e)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    fetch()
    return () => {
      cancelled = true
    }
  }, [portfolioId, displayCurrency])

  const assets: Record<AssetId, number> = { stocks: 0, bonds: 0, cash: 0, commodities: 0 }
  holdings.forEach((h) => {
    assets[h.assetId] = (assets[h.assetId] || 0) + (h.value || 0)
  })

  const addHolding = useCallback(
    async (holding: Omit<Holding, "id">) => {
      if (!portfolioId) return
      const result = await api.createHolding(portfolioId, holding)
      setHoldings((prev) => {
        const idx = prev.findIndex((h) => h.id === result.id)
        if (idx >= 0) {
          const updated = [...prev]
          updated[idx] = result
          return updated
        }
        return [...prev, result]
      })
    },
    [portfolioId]
  )

  const updateHolding = useCallback(
    async (id: string, updates: Partial<Holding>) => {
      if (!portfolioId) return
      try {
        const result = await api.updateHolding(portfolioId, id, updates)
        setHoldings((prev) => prev.map((h) => (h.id === id ? result : h)))
      } catch (e) {
        console.error("Failed to update holding", e)
      }
    },
    [portfolioId]
  )

  const removeHolding = useCallback(
    async (id: string) => {
      if (!portfolioId) return
      try {
        await api.deleteHolding(portfolioId, id)
        setHoldings((prev) => prev.filter((h) => h.id !== id))
      } catch (e) {
        console.error("Failed to remove holding", e)
      }
    },
    [portfolioId]
  )

  const saveRecord = useCallback(async () => {
    if (!portfolioId) return
    try {
      const result = await api.createRecord(portfolioId, displayCurrency)
      setHistory((prev) => [result, ...prev])
    } catch (e) {
      console.error("Failed to save record", e)
    }
  }, [portfolioId, displayCurrency])

  const deleteRecord = useCallback(
    async (id: string) => {
      if (!portfolioId) return
      try {
        await api.deleteRecord(portfolioId, id)
        setHistory((prev) => prev.filter((r) => r.id !== id))
      } catch (e) {
        console.error("Failed to delete record", e)
      }
    },
    [portfolioId]
  )

  return {
    holdings,
    setHoldings,
    assets,
    history,
    loading,
    addHolding,
    updateHolding,
    removeHolding,
    saveRecord,
    deleteRecord,
  }
}
