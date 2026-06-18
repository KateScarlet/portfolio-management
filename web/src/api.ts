import { Holding, PortfolioRecord } from './types';

const BASE = '';

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(BASE + url, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
}

export async function fetchHoldings(): Promise<Holding[]> {
  return request<Holding[]>('/api/holdings');
}

export async function createHolding(h: Omit<Holding, 'id'>): Promise<Holding> {
  return request<Holding>('/api/holdings', {
    method: 'POST',
    body: JSON.stringify(h),
  });
}

export async function updateHolding(id: string, updates: Partial<Holding>): Promise<Holding> {
  return request<Holding>(`/api/holdings/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(updates),
  });
}

export async function deleteHolding(id: string): Promise<void> {
  await request<{ success: boolean }>(`/api/holdings/${id}`, {
    method: 'DELETE',
  });
}

export async function sellHolding(
  id: string,
  shares: number,
  price: number
): Promise<{ holding: Holding; cashHolding: Holding }> {
  return request<{ holding: Holding; cashHolding: Holding }>(`/api/holdings/${id}/sell`, {
    method: 'POST',
    body: JSON.stringify({ shares, price }),
  });
}

export async function fetchRecords(): Promise<PortfolioRecord[]> {
  return request<PortfolioRecord[]>('/api/records');
}

export async function createRecord(): Promise<PortfolioRecord> {
  return request<PortfolioRecord>('/api/records', {
    method: 'POST',
  });
}

export async function deleteRecord(id: string): Promise<void> {
  await request<{ success: boolean }>(`/api/records/${id}`, {
    method: 'DELETE',
  });
}
