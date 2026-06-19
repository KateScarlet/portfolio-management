import React from 'react';
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts';
import { AssetId, ASSET_DEFINITIONS } from '../types';
import { formatCurrency, formatPercent } from '../utils';

interface DashboardProps {
  assets: Record<AssetId, number>;
  total: number;
  principal: number;
  totalFees: number;
}

export default function Dashboard({ assets, total, principal, totalFees }: DashboardProps) {
  const chartData = Object.keys(assets).map((key) => {
    const id = key as AssetId;
    const value = assets[id];
    return {
      name: ASSET_DEFINITIONS[id].name,
      value,
      color: ASSET_DEFINITIONS[id].color,
    };
  }).filter((item) => item.value > 0);

  const profit = total - principal;
  const returnRate = principal > 0 ? profit / principal : 0;
  const isPositive = profit >= 0;

  return (
    <div className="bg-white p-6 sm:p-8 rounded-2xl border border-[#E9ECEF] shadow-sm flex flex-col h-full">
      <div className="flex items-center justify-between mb-2">
        <p className="text-xs uppercase tracking-[0.2em] text-[#6C757D] font-semibold">总资产净值</p>
        
        <div className="flex items-center gap-2 text-xs text-[#ADB5BD]">
           <span>累计投入成本: <span className="font-mono">{formatCurrency(principal)}</span></span>
           {totalFees > 0 && <span className="ml-2">累计手续费: <span className="font-mono">{formatCurrency(totalFees)}</span></span>}
        </div>
      </div>
      
      <h2 className="text-4xl font-light mb-2 text-[#1A1A1A]">
        {formatCurrency(total)}
      </h2>

      <div className={`flex items-center gap-2 mb-8 ${isPositive ? 'text-emerald-600' : 'text-orange-600'} font-mono`}>
        {isPositive ? (
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6"></path></svg>
        ) : (
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 17h8m0 0v-8m0 8l-8-8-4 4-6-6"></path></svg>
        )}
        <span className="text-sm font-medium">
          {isPositive ? '+' : ''}{formatPercent(returnRate)} ({isPositive ? '+' : ''}{formatCurrency(profit)})
        </span>
      </div>

      <div className="w-full h-64 relative flex justify-center">
        {total > 0 ? (
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={chartData}
                cx="50%"
                cy="50%"
                innerRadius={70}
                outerRadius={95}
                paddingAngle={2}
                dataKey="value"
                stroke="none"
              >
                {chartData.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={entry.color} />
                ))}
              </Pie>
              <Tooltip
                formatter={(value) => formatCurrency(Number(value))}
                contentStyle={{ borderRadius: '0.5rem', border: '1px solid #E9ECEF', boxShadow: '0 4px 6px -1px rgb(0 0 0 / 0.05)', backgroundColor: '#fff' }}
              />
            </PieChart>
          </ResponsiveContainer>
        ) : (
          <div className="absolute inset-0 flex flex-col items-center justify-center">
            <p className="text-2xl font-light text-[#1A1A1A]">0.00</p>
            <p className="text-[10px] text-[#ADB5BD] uppercase tracking-wider mt-1">暂无数据</p>
          </div>
        )}
        {total > 0 && (
          <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
            <p className="text-2xl font-light text-[#1A1A1A]">100%</p>
            <p className="text-[10px] text-[#ADB5BD] uppercase tracking-wider mt-1">Balanced</p>
          </div>
        )}
      </div>

      <div className="mt-8 grid grid-cols-2 gap-4">
        {Object.keys(ASSET_DEFINITIONS).map((key) => {
          const id = key as AssetId;
          const value = assets[id] || 0;
          const percentage = total > 0 ? value / total : 0;
          const def = ASSET_DEFINITIONS[id];

          return (
            <div key={id} className="flex items-center gap-3">
              <div className={`w-2 h-2 rounded-full ${id === 'cash' ? 'border border-[#ADB5BD]' : ''}`} style={{ backgroundColor: def.color }} />
              <span className="text-xs text-[#495057] font-medium">{def.name} <span className="font-normal text-[#ADB5BD]">({formatPercent(percentage)})</span></span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
