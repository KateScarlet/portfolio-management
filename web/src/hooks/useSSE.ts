import { useEffect, useRef, useCallback, useState } from "react"
import type { SSEEvent } from "../types/events"

interface UseSSEOptions {
  onEvent: (event: SSEEvent) => void
  enabled?: boolean
}

const MAX_RETRY_DELAY = 30000
const BASE_RETRY_DELAY = 1000

export function useSSE({ onEvent, enabled = true }: UseSSEOptions) {
  const [connected, setConnected] = useState(false)
  const esRef = useRef<EventSource | null>(null)
  const retryCountRef = useRef(0)
  const onEventRef = useRef(onEvent)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    onEventRef.current = onEvent
  }, [onEvent])

  const connect = useCallback(() => {
    if (esRef.current) {
      esRef.current.close()
    }

    const es = new EventSource("/api/events")
    esRef.current = es

    es.addEventListener("connected", () => {
      setConnected(true)
      retryCountRef.current = 0
    })

    es.addEventListener("sync.started", (e) => {
      onEventRef.current(JSON.parse((e as MessageEvent).data))
    })

    es.addEventListener("sync.completed", (e) => {
      onEventRef.current(JSON.parse((e as MessageEvent).data))
    })

    es.addEventListener("sync.failed", (e) => {
      onEventRef.current(JSON.parse((e as MessageEvent).data))
    })

    es.addEventListener("price.updated", (e) => {
      onEventRef.current(JSON.parse((e as MessageEvent).data))
    })

    es.onerror = () => {
      setConnected(false)
      es.close()
      esRef.current = null

      const delay = Math.min(
        BASE_RETRY_DELAY * Math.pow(2, retryCountRef.current),
        MAX_RETRY_DELAY
      )
      retryCountRef.current++
      reconnectTimerRef.current = setTimeout(connect, delay)
    }
  }, [])

  const disconnect = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
    if (esRef.current) {
      esRef.current.close()
      esRef.current = null
    }
    setConnected(false)
  }, [])

  useEffect(() => {
    if (!enabled) {
      disconnect()
      return
    }
    connect()
    return disconnect
  }, [enabled, connect, disconnect])

  useEffect(() => {
    const handleVisibility = () => {
      if (document.visibilityState === "visible" && enabled && !esRef.current) {
        connect()
      }
    }
    document.addEventListener("visibilitychange", handleVisibility)
    return () => document.removeEventListener("visibilitychange", handleVisibility)
  }, [enabled, connect])

  return { connected }
}
