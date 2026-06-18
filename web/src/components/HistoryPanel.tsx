import React from 'react';
import { format } from 'date-fns';
import { Trash2 } from 'lucide-react';
import { PortfolioRecord } from '../types';
import { formatCurrency, formatPercent } from '../utils';

interface HistoryPanelProps {
  history: PortfolioRecord[];
  onDeleteRecord: (id: string) => void;
}

export default function HistoryPanel({ history, onDeleteRecord }: HistoryPanelProps) {
  if (history.length === 0) {
    return null;
  }

  return (
    <div className="bg-white rounded-2xl border border-[#E9ECEF] shadow-sm flex flex-col overflow-hidden">
      <div className="p-6 border-b border-[#F1F3F5]">
        <h3 className="text-lg font-medium text-[#1A1A1A]">历史快照与盈亏记录</h3>
      </div>
      
      <div className="flex-grow overflow-x-auto">
        <table className="w-full text-left">
          <thead className="text-[10px] uppercase tracking-widest text-[#ADB5BD] border-b border-[#F1F3F5] bg-white">
            <tr>
              <th className="px-6 py-4 font-bold">日期</th>
              <th className="px-6 py-4 font-bold">总价值</th>
              <th className="px-6 py-4 font-bold">总成本</th>
              <th className="px-6 py-4 font-bold">累计盈亏</th>
              <th className="px-6 py-4 font-bold">股票</th>
              <th className="px-6 py-4 font-bold">债券</th>
              <th className="px-6 py-4 font-bold">商品</th>
              <th className="px-6 py-4 font-bold">现金</th>
              <th className="px-6 py-4 font-bold text-right">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[#F8F9FA] bg-white text-[#1A1A1A]">
            {history.map((record) => {
              const principal = record.principal || 0;
              const profit = record.total - principal;
              const returnRate = principal > 0 ? profit / principal : 0;
              const isPositive = profit >= 0;

              return (
                <tr key={record.id} className="hover:bg-[#F8F9FA] transition-colors">
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-[#6C757D]">
                    {format(record.timestamp, 'MM/dd, HH:mm')}
                  </td>
                  <td className="px-6 py-4 font-mono text-sm font-medium">
                    {formatCurrency(record.total)}
                  </td>
                  <td className="px-6 py-4 font-mono text-sm text-[#495057]">
                    {principal > 0 ? formatCurrency(principal) : '-'}
                  </td>
                  <td className="px-6 py-4 font-mono text-sm">
                    {principal > 0 ? (
                      <span className={isPositive ? 'text-emerald-600' : 'text-orange-600'}>
                        {isPositive ? '+' : ''}{formatCurrency(profit)}
                        <br/>
                        <span className="text-[10px] opacity-80">{isPositive ? '+' : ''}{formatPercent(returnRate)}</span>
                      </span>
                    ) : (
                      <span className="text-[#ADB5BD]">-</span>
                    )}
                  </td>
                  <td className="px-6 py-4 text-xs font-mono text-[#ADB5BD]">{formatCurrency(record.assets.stocks || 0)}</td>
                  <td className="px-6 py-4 text-xs font-mono text-[#ADB5BD]">{formatCurrency(record.assets.bonds || 0)}</td>
                  <td className="px-6 py-4 text-xs font-mono text-[#ADB5BD]">{formatCurrency(record.assets.gold || 0)}</td>
                  <td className="px-6 py-4 text-xs font-mono text-[#ADB5BD]">{formatCurrency(record.assets.cash || 0)}</td>
                  <td className="px-6 py-4 text-right">
                    <button
                      onClick={() => onDeleteRecord(record.id)}
                      className="text-[10px] uppercase tracking-wider text-[#ADB5BD] hover:text-orange-500 font-bold transition-colors"
                      title="删除记录"
                    >
                      Del
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
