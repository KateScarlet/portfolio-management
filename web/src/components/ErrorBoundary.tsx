import { Component, type ReactNode } from "react"

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex min-h-screen items-center justify-center bg-[#F8F9FA] p-8">
          <div className="max-w-md rounded-xl border border-[#E9ECEF] bg-white p-8 text-center shadow-sm">
            <div className="mb-4 text-4xl">!</div>
            <h1 className="mb-2 text-lg font-bold text-[#1A1A1A]">应用出现错误</h1>
            <p className="mb-4 text-sm text-[#6C757D]">
              {this.state.error?.message || "发生了未知错误"}
            </p>
            <button
              onClick={() => window.location.reload()}
              className="rounded-lg bg-[#1A1A1A] px-4 py-2 text-sm font-bold text-white transition-colors hover:bg-[#495057]"
            >
              刷新页面
            </button>
          </div>
        </div>
      )
    }

    return this.props.children
  }
}
