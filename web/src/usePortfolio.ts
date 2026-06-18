import { useState, useEffect } from 'react';
import { AssetId, PortfolioRecord, Holding } from './types';

const HOLDINGS_KEY = 'permanent-portfolio-holdings';
const HISTORY_KEY = 'permanent-portfolio-history';
const PRINCIPAL_KEY = 'permanent-portfolio-principal';

export function usePortfolio() {
  const [holdings, setHoldings] = useState<Holding[]>(() => {
    try {
      const stored = localStorage.getItem(HOLDINGS_KEY);
      if (stored) {
        return JSON.parse(stored);
      }
      
      // Migration from old app
      const oldAssetsStr = localStorage.getItem('permanent-portfolio-assets');
      if (oldAssetsStr) {
        const oldAssets = JSON.parse(oldAssetsStr);
        const initialHoldings: Holding[] = [];
        for (const key of Object.keys(oldAssets)) {
          if (oldAssets[key] > 0) {
            initialHoldings.push({
              id: crypto.randomUUID(),
              assetId: key as AssetId,
              symbol: '',
              shares: 0,
              price: 0,
              value: oldAssets[key]
            });
          }
        }
        return initialHoldings;
      }
    } catch (e) {
      console.error('Failed to load holdings', e);
    }
    return [];
  });

  const [history, setHistory] = useState<PortfolioRecord[]>(() => {
    try {
      const stored = localStorage.getItem(HISTORY_KEY);
      if (stored) {
        return JSON.parse(stored);
      }
    } catch (e) {
      console.error('Failed to load history', e);
    }
    return [];
  });

  useEffect(() => {
    localStorage.setItem(HOLDINGS_KEY, JSON.stringify(holdings));
  }, [holdings]);

  useEffect(() => {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(history));
  }, [history]);

  const assets: Record<AssetId, number> = { stocks: 0, bonds: 0, cash: 0, gold: 0 };
  holdings.forEach(h => {
    assets[h.assetId] = (assets[h.assetId] || 0) + (h.value || 0);
  });

  const addHolding = (holding: Omit<Holding, 'id'>) => {
    setHoldings((prev) => {
      // Try to find an existing holding by symbol or (for manual assets) by name and assetId
      const existingIdx = prev.findIndex(h => 
        (holding.symbol && h.symbol === holding.symbol) || 
        (!holding.symbol && !h.symbol && h.name === holding.name && h.assetId === holding.assetId)
      );

      const newLot = {
        id: crypto.randomUUID(),
        date: holding.date || Date.now(),
        shares: holding.shares,
        costPrice: holding.costPrice,
        cost: holding.cost,
        valueAdded: holding.value // initial value added
      };

      if (existingIdx >= 0) {
        const existing = prev[existingIdx];
        const updated = { ...existing };
        
        updated.shares += holding.shares;
        
        if (updated.cost !== undefined && holding.cost !== undefined) {
           updated.cost += holding.cost;
        } else if (holding.cost !== undefined) {
           updated.cost = holding.cost;
        } else if (updated.cost === undefined && !holding.symbol) {
           // For manual assets, if they add value but we don't have cost, try to add value to cost
           const manualCost = holding.cost || holding.value;
           updated.cost = manualCost;
        }
        
        if (updated.shares > 0 && updated.cost !== undefined) {
           updated.costPrice = updated.cost / updated.shares;
        }
        
        if (updated.symbol && holding.price) {
          // Automatic pricing update
          updated.price = holding.price;
          updated.value = updated.shares * updated.price;
        } else {
          updated.value += holding.value; // For manual, add value cumulatively
        }

        updated.lots = [...(existing.lots || []), newLot];

        const newHoldings = [...prev];
        newHoldings[existingIdx] = updated;
        return newHoldings;
      } else {
        return [...prev, { ...holding, id: crypto.randomUUID(), lots: [newLot] }];
      }
    });
  };

  const updateHolding = (id: string, updates: Partial<Holding>) => {
    setHoldings((prev) =>
      prev.map((h) => {
        if (h.id === id) {
          const updated = { ...h, ...updates };
          // Auto-calculate value if we have shares and price
          if (updated.symbol && updated.shares !== undefined && updated.price !== undefined) {
             updated.value = updated.shares * updated.price;
          }
          return updated;
        }
        return h;
      })
    );
  };

  const removeHolding = (id: string) => {
    setHoldings((prev) => prev.filter((h) => h.id !== id));
  };

  const saveRecord = () => {
    const total = Object.values(assets).reduce((sum, val) => sum + val, 0);
    const totalCost = holdings.reduce((sum, h) => sum + (h.cost || 0), 0);
    if (total === 0) return;

    const newRecord: PortfolioRecord = {
      id: crypto.randomUUID(),
      timestamp: Date.now(),
      assets: { ...assets },
      total,
      principal: totalCost,
    };

    setHistory((prev) => {
      const newHistory = [newRecord, ...prev].sort((a, b) => b.timestamp - a.timestamp);
      return newHistory;
    });
  };

  const deleteRecord = (id: string) => {
    setHistory((prev) => prev.filter((r) => r.id !== id));
  };

  return {
    holdings,
    assets,
    history,
    addHolding,
    updateHolding,
    removeHolding,
    saveRecord,
    deleteRecord,
  };
}
