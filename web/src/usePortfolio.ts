import { useState, useEffect, useCallback } from "react"
import { AssetId, Holding, PortfolioRecord } from "./types"
import * as api from "./api"

export function usePortfolio() {
  const [holdings, setHoldings] = useState<Holding[]>([])
  const [history, setHistory] = useState<PortfolioRecord[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    Promise.all([api.fetchHoldings(), api.fetchRecords()])
      .then(([h, r]) => {
        setHoldings(h)
        setHistory(r)
      })
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  const assets: Record<AssetId, number> = { stocks: 0, bonds: 0, cash: 0, gold: 0 }
  holdings.forEach((h) => {
    assets[h.assetId] = (assets[h.assetId] || 0) + (h.value || 0)
  })

  const addHolding = useCallback(async (holding: Omit<Holding, "id">) => {
    const result = await api.createHolding(holding)
    setHoldings((prev) => {
      const idx = prev.findIndex((h) => h.id === result.id)
      if (idx >= 0) {
        const updated = [...prev]
        updated[idx] = result
        return updated
      }
      return [...prev, result]
    })
  }, [])

  const updateHolding = useCallback(async (id: string, updates: Partial<Holding>) => {
    try {
      const result = await api.updateHolding(id, updates)
      setHoldings((prev) => prev.map((h) => (h.id === id ? result : h)))
    } catch (e) {
      console.error("Failed to update holding", e)
    }
  }, [])

  const removeHolding = useCallback(async (id: string) => {
    try {
      await api.deleteHolding(id)
      setHoldings((prev) => prev.filter((h) => h.id !== id))
    } catch (e) {
      console.error("Failed to delete holding", e)
    }
  }, [])

  const saveRecord = useCallback(async () => {
    try {
      const result = await api.createRecord()
      setHistory((prev) => [result, ...prev])
    } catch (e) {
      console.error("Failed to save record", e)
    }
  }, [])

  const deleteRecord = useCallback(async (id: string) => {
    try {
      await api.deleteRecord(id)
      setHistory((prev) => prev.filter((r) => r.id !== id))
    } catch (e) {
      console.error("Failed to delete record", e)
    }
  }, [])

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
