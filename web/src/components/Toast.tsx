import { useState, useCallback, ReactNode } from "react"
import { ToastContext, ToastContextValue } from "./toast-context"

interface Toast {
  id: number
  message: string
  type: "error" | "success" | "info"
}

let nextId = 0

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])

  const showToast = useCallback((message: string, type: Toast["type"] = "error") => {
    const id = nextId++
    setToasts((prev) => [...prev, { id, message, type }])
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id))
    }, 4000)
  }, [])

  const value: ToastContextValue = { showToast }

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="fixed bottom-6 right-6 z-50 flex flex-col gap-2 pointer-events-none">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`pointer-events-auto px-4 py-3 rounded-xl shadow-lg text-sm font-medium transition-all animate-in fade-in slide-in-from-bottom-2 ${
              t.type === "error"
                ? "bg-orange-500 text-white"
                : t.type === "success"
                  ? "bg-emerald-600 text-white"
                  : "bg-[#1A1A1A] text-white"
            }`}
          >
            {t.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}
