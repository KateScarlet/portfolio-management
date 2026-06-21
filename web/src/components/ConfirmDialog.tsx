interface ConfirmDialogProps {
  title: string
  message: string
  onConfirm: () => void
  onCancel: () => void
}

export default function ConfirmDialog({ title, message, onConfirm, onCancel }: ConfirmDialogProps) {
  return (
    <div
      className="fixed inset-0 bg-[#1A1A1A]/80 z-50 flex items-center justify-center p-4 backdrop-blur-sm"
      onClick={onCancel}
    >
      <div
        className="bg-white rounded-2xl max-w-sm w-full p-6 shadow-2xl flex flex-col gap-6"
        onClick={(e) => e.stopPropagation()}
      >
        <div>
          <h3 className="text-lg font-bold text-[#1A1A1A]">{title}</h3>
          <p className="text-sm text-[#6C757D] mt-1">{message}</p>
        </div>
        <div className="flex gap-3 justify-end pt-2 border-t border-[#F1F3F5]">
          <button
            onClick={onCancel}
            className="px-4 py-2 text-sm font-medium text-[#6C757D] hover:bg-[#F8F9FA] rounded-xl transition-colors"
          >
            取消
          </button>
          <button
            onClick={onConfirm}
            className="px-4 py-2 text-sm font-medium text-white bg-orange-500 hover:bg-orange-600 rounded-xl transition-colors shadow-sm"
          >
            确认
          </button>
        </div>
      </div>
    </div>
  )
}
