import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import ErrorBoundary from "./components/ErrorBoundary.tsx"
import { ToastProvider } from "./components/Toast.tsx"
import App from "./App.tsx"
import "./index.css"

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ErrorBoundary>
      <ToastProvider>
        <App />
      </ToastProvider>
    </ErrorBoundary>
  </StrictMode>
)
