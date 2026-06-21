import { useState } from "react"
import { Settings } from "../types"
import { Settings as SettingsIcon } from "lucide-react"
import * as api from "../api"

interface SettingsPanelProps {
  settings: Settings
  onSave: (settings: Settings) => void
}

const SYNC_PRESETS = [
  { value: 0, label: "关闭" },
  { value: 30, label: "30分钟" },
  { value: 60, label: "1小时" },
  { value: 120, label: "2小时" },
  { value: 240, label: "4小时" },
]

const SUMMARY_INTERVALS = [
  { value: "off", label: "关闭" },
  { value: "daily", label: "每日" },
  { value: "weekly", label: "每周" },
]

export default function SettingsPanel({ settings, onSave }: SettingsPanelProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [draft, setDraft] = useState(settings)
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null)

  const handleSave = () => {
    onSave(draft)
    setIsOpen(false)
    setTestResult(null)
  }

  const handleTestConnection = async () => {
    if (!draft.telegramBotToken || !draft.telegramChatID) {
      setTestResult({ success: false, message: "请先填写 Bot Token 和 Chat ID" })
      return
    }
    setTesting(true)
    setTestResult(null)
    try {
      const result = await api.testTelegramConnection(draft.telegramBotToken, draft.telegramChatID)
      if (result.success) {
        setTestResult({ success: true, message: `连接成功！Bot: @${result.botName}` })
      } else {
        setTestResult({ success: false, message: result.error || "连接失败" })
      }
    } catch (e) {
      setTestResult({ success: false, message: "连接失败: " + (e instanceof Error ? e.message : "未知错误") })
    } finally {
      setTesting(false)
    }
  }

  const presets = [3, 5, 7, 10, 15, 20]

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
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/20"
          onClick={() => setIsOpen(false)}
        >
          <div
            className="bg-white rounded-2xl shadow-xl w-full max-w-md mx-4 max-h-[80vh] flex flex-col"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Fixed Header */}
            <div className="flex items-center justify-between px-6 pt-6 pb-4">
              <h3 className="text-lg font-medium text-[#1A1A1A]">设置</h3>
              <button
                onClick={() => setIsOpen(false)}
                className="text-[#ADB5BD] hover:text-[#1A1A1A] text-xl leading-none"
              >
                &times;
              </button>
            </div>

            {/* Scrollable Content */}
            <div className="px-6 pb-2 space-y-6 overflow-y-auto scrollbar-thin flex-1 min-h-0">
              {/* Drift Threshold */}
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
                      onChange={(e) =>
                        setDraft({
                          ...draft,
                          driftThreshold: Math.max(1, Math.min(30, Number(e.target.value) || 1)),
                        })
                      }
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
                          ? "bg-[#1A1A1A] text-white border-[#1A1A1A]"
                          : "bg-white text-[#6C757D] border-[#E9ECEF] hover:border-[#ADB5BD]"
                      }`}
                    >
                      {p}%
                    </button>
                  ))}
                </div>
              </div>

              {/* Sync Interval */}
              <div>
                <label className="block text-sm font-medium text-[#1A1A1A] mb-2">
                  自动同步价格
                </label>
                <p className="text-xs text-[#6C757D] mb-3">定时从 Yahoo Finance 获取最新价格。</p>
                <div className="flex flex-wrap gap-2">
                  {SYNC_PRESETS.map((p) => (
                    <button
                      key={p.value}
                      onClick={() => setDraft({ ...draft, syncInterval: p.value })}
                      className={`px-3 py-1 text-xs rounded-full border transition-colors ${
                        draft.syncInterval === p.value
                          ? "bg-[#1A1A1A] text-white border-[#1A1A1A]"
                          : "bg-white text-[#6C757D] border-[#E9ECEF] hover:border-[#ADB5BD]"
                      }`}
                    >
                      {p.label}
                    </button>
                  ))}
                </div>
              </div>

              {/* Telegram Notification */}
              <div className="border-t border-[#E9ECEF] pt-6">
                <div className="flex items-center justify-between mb-4">
                  <div>
                    <label className="block text-sm font-medium text-[#1A1A1A]">
                      Telegram 通知
                    </label>
                    <p className="text-xs text-[#6C757D] mt-1">
                      通过 Telegram Bot 接收投资组合通知
                    </p>
                  </div>
                  <button
                    onClick={() => setDraft({ ...draft, telegramEnabled: !draft.telegramEnabled })}
                    className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                      draft.telegramEnabled ? "bg-[#1A1A1A]" : "bg-[#E9ECEF]"
                    }`}
                  >
                    <span
                      className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                        draft.telegramEnabled ? "translate-x-6" : "translate-x-1"
                      }`}
                    />
                  </button>
                </div>

                {draft.telegramEnabled && (
                  <div className="space-y-4 mt-4">
                    {/* Bot Token */}
                    <div>
                      <label className="block text-xs font-medium text-[#6C757D] mb-1">
                        Bot Token
                      </label>
                      <input
                        type="password"
                        value={draft.telegramBotToken}
                        onChange={(e) => setDraft({ ...draft, telegramBotToken: e.target.value })}
                        placeholder="从 @BotFather 获取"
                        className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                      />
                    </div>

                    {/* Chat ID */}
                    <div>
                      <label className="block text-xs font-medium text-[#6C757D] mb-1">
                        Chat ID
                      </label>
                      <input
                        type="text"
                        value={draft.telegramChatID}
                        onChange={(e) => setDraft({ ...draft, telegramChatID: e.target.value })}
                        placeholder="发送 /start 给 @userinfobot 获取"
                        className="w-full px-3 py-2 text-sm border border-[#E9ECEF] rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1A1A1A] focus:border-transparent"
                      />
                    </div>

                    {/* Test Connection */}
                    <div className="flex items-center gap-3">
                      <button
                        onClick={handleTestConnection}
                        disabled={testing}
                        className="px-4 py-2 text-sm text-[#1A1A1A] border border-[#E9ECEF] rounded-lg hover:bg-[#F1F3F5] transition-colors disabled:opacity-50"
                      >
                        {testing ? "测试中..." : "测试连接"}
                      </button>
                      {testResult && (
                        <span
                          className={`text-xs ${
                            testResult.success ? "text-green-600" : "text-red-500"
                          }`}
                        >
                          {testResult.message}
                        </span>
                      )}
                    </div>

                    {/* Notification Toggles */}
                    <div className="space-y-3 pt-2">
                      <label className="block text-xs font-medium text-[#6C757D]">
                        通知类型
                      </label>

                      {/* Price Alert */}
                      <div className="flex items-center justify-between">
                        <div>
                          <span className="text-sm text-[#1A1A1A]">价格大幅波动</span>
                          <div className="flex items-center gap-2 mt-1">
                            <span className="text-xs text-[#6C757D]">阈值:</span>
                            <input
                              type="number"
                              min="1"
                              max="50"
                              value={draft.telegramPriceThreshold}
                              onChange={(e) =>
                                setDraft({
                                  ...draft,
                                  telegramPriceThreshold: Math.max(
                                    1,
                                    Math.min(50, Number(e.target.value) || 1)
                                  ),
                                })
                              }
                              className="w-12 px-2 py-1 text-xs text-center border border-[#E9ECEF] rounded focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
                            />
                            <span className="text-xs text-[#6C757D]">%</span>
                          </div>
                        </div>
                        <button
                          onClick={() =>
                            setDraft({ ...draft, telegramPriceAlert: !draft.telegramPriceAlert })
                          }
                          className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
                            draft.telegramPriceAlert ? "bg-[#1A1A1A]" : "bg-[#E9ECEF]"
                          }`}
                        >
                          <span
                            className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform ${
                              draft.telegramPriceAlert ? "translate-x-4.5" : "translate-x-0.5"
                            }`}
                          />
                        </button>
                      </div>

                      {/* Drift Alert */}
                      <div className="flex items-center justify-between">
                        <span className="text-sm text-[#1A1A1A]">配比偏离提醒</span>
                        <button
                          onClick={() =>
                            setDraft({ ...draft, telegramDriftAlert: !draft.telegramDriftAlert })
                          }
                          className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
                            draft.telegramDriftAlert ? "bg-[#1A1A1A]" : "bg-[#E9ECEF]"
                          }`}
                        >
                          <span
                            className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform ${
                              draft.telegramDriftAlert ? "translate-x-4.5" : "translate-x-0.5"
                            }`}
                          />
                        </button>
                      </div>

                      {/* Summary */}
                      <div className="flex items-center justify-between">
                        <span className="text-sm text-[#1A1A1A]">定期组合摘要</span>
                        <select
                          value={draft.telegramSummaryInterval}
                          onChange={(e) =>
                            setDraft({ ...draft, telegramSummaryInterval: e.target.value })
                          }
                          className="px-2 py-1 text-xs border border-[#E9ECEF] rounded focus:outline-none focus:ring-1 focus:ring-[#1A1A1A]"
                        >
                          {SUMMARY_INTERVALS.map((opt) => (
                            <option key={opt.value} value={opt.value}>
                              {opt.label}
                            </option>
                          ))}
                        </select>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            </div>

            {/* Fixed Footer */}
            <div className="flex justify-end gap-3 px-6 py-4 border-t border-[#E9ECEF]">
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
  )
}
