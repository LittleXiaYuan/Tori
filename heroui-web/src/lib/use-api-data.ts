import { useState, useCallback, useEffect, useRef } from "react";

/**
 * Generic data-fetching hook with automatic cancellation on unmount
 * to prevent state updates on stale / unmounted components during
 * rapid navigation.
 */
export function useApiData<T>(
  fetcher: () => Promise<T>,
  initial: T,
  deps: unknown[] = [],
) {
  const [data, setData] = useState<T>(initial);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;
  const mountedRef = useRef(true);
  const seqRef = useRef(0);

  useEffect(() => {
    mountedRef.current = true;
    return () => { mountedRef.current = false; };
  }, []);

  const load = useCallback(async () => {
    const seq = ++seqRef.current;
    try {
      const result = await fetcherRef.current();
      if (mountedRef.current && seq === seqRef.current) {
        setData(result);
        setError(null);
      }
    } catch (e) {
      if (mountedRef.current && seq === seqRef.current) {
        setError(e instanceof Error ? e : new Error(String(e)));
      }
    } finally {
      if (mountedRef.current && seq === seqRef.current) {
        setLoading(false);
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  useEffect(() => {
    load();
  }, [load]);

  const refresh = useCallback(() => {
    setLoading(true);
    load();
  }, [load]);

  return { data, setData, loading, error, refresh } as const;
}
