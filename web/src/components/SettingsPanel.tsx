import React, { useState } from 'react';
import { Settings } from '../types';
import { Settings as SettingsIcon } from 'lucide-react';

interface SettingsPanelProps {
  settings: Settings;
  onSave: (settings: Settings) => void;
}

export default function SettingsPanel({ settings, onSave }: SettingsPanelProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [draft, setDraft] = useState(settings);

  const handleSave = () => {
    onSave(draft);
    setIsOpen(false);
  };

  const presets = [3, 5, 7, 10, 15, 20];

  return (
    <>
      <button
        onClick={() => setIsOpen(true)}
        className="p-2 rounded-lg hover:bg-[#F1F3F5] transition-colors text-[#6C757D] hover:text-[#1A1A1A]"
        title="设置"
      >
        <SettingsIcon className="w-5 h-5" />
      </button>

      {isOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/20" onClick={() => setIsOpen(false)}>
          <div className="bg-white rounded-2xl shadow-xl w-full max-w-md mx-4 p-6" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-6">
              <h3 className="text-lg font-medium text-[#1A1A1A]">设置</h3>
              <button onClick={() => setIsOpen(false)} className="text-[#ADB5BD] hover:text-[#1A1A1A] text-xl leading-none">&times;</button>
            </div>

            <div className="space-y-6">
              <div>
                <label className="block text-sm font-medium text-[#1A1A1A] mb-2">
                  再平衡漂移阈值
                </label>
                <p className="text-xs text-[#6C757D] mb-3">
                  当资产偏离目标配比超过此阈值时，提示需要再平衡。
                </p>
                <div className="flex items-center gap-3">
                  <input
                    type="range"
                    min="1"
                    max="30"
                    step="1"
                    value={draft.driftThreshold}
                    onChange={(e) => setDraft({ ...draft, driftThreshold: Number(e.target.value) })}
                    className="flex-1 h-2 bg-[#E9ECEF] rounded-lg appearance-none cursor-pointer accent-[#1A1A1A]"
                  />
                  <div className="flex items-center gap-1 w-20">
                    <input
                      type="number"
                      min="1"
                      max="30"
                      value={draft.driftThreshold}
                      onChange={(e) => setDraft({ ...draft, driftThreshold: Math.max(1, Math.min(30, Number(e.target.value) || 1)) })}
                      className="w-14 px-2 py-1.5 text-sm text-center border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                    />
                    <span className="text-xs text-[#6C757D]">%</span>
                  </div>
                </div>
                <div className="flex flex-wrap gap-2 mt-3">
                  {presets.map((p) => (
                    <button
                      key={p}
                      onClick={() => setDraft({ ...draft, driftThreshold: p })}
                      className={`px-3 py-1 text-xs rounded-full border transition-colors ${
                        draft.driftThreshold === p
                          ? 'bg-[#1A1A1A] text-white border-[#1A1A1A]'
                          : 'bg-white text-[#6C757D] border-[#E9ECEF] hover:border-[#ADB5BD]'
                      }`}
                    >
                      {p}%
                    </button>
                  ))}
                </div>
              </div>
            </div>

            <div className="flex justify-end gap-3 mt-8">
              <button
                onClick={() => setIsOpen(false)}
                className="px-4 py-2 text-sm text-[#6C757D] hover:text-[#1A1A1A] transition-colors"
              >
                取消
              </button>
              <button
                onClick={handleSave}
                className="px-4 py-2 text-sm bg-[#1A1A1A] text-white rounded-lg hover:bg-[#333] transition-colors"
              >
                保存
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
