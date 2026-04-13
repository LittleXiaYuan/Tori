import { useEffect, useRef } from "react";

/**
 * Polling hook that pauses when the tab is not visible.
 * @param callback  async function to call
 * @param interval  polling interval in ms
 * @param enabled   whether polling is active (default true)
 */
export function usePolling(callback: () => void | Promise<void>, interval: number, enabled = true) {
  const savedCb = useRef(callback);
  savedCb.current = callback;

  useEffect(() => {
    if (!enabled || interval <= 0) return;

    let timer: ReturnType<typeof setInterval> | null = null;

    const start = () => {
      if (timer) return;
      timer = setInterval(() => savedCb.current(), interval);
    };
    const stop = () => {
      if (timer) { clearInterval(timer); timer = null; }
    };

    const onVisChange = () => {
      if (document.visibilityState === "visible") start();
      else stop();
    };

    // Start immediately if visible
    if (document.visibilityState === "visible") start();
    document.addEventListener("visibilitychange", onVisChange);

    return () => {
      stop();
      document.removeEventListener("visibilitychange", onVisChange);
    };
  }, [interval, enabled]);
}
