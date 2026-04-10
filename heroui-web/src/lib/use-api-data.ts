import { useState, useCallback, useEffect, useRef } from "react";

/**
 * Generic data-fetching hook that replaces the repeated pattern of
 *   const [data, setData] = useState(initial);
 *   const [loading, setLoading] = useState(true);
 *   const load = useCallback(async () => { ... setLoading(false); }, []);
 *   useEffect(() => { load(); }, [load]);
 */
export function useApiData<T>(
  fetcher: () => Promise<T>,
  initial: T,
  deps: unknown[] = [],
) {
  const [data, setData] = useState<T>(initial);
  const [loading, setLoading] = useState(true);
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;

  const load = useCallback(async () => {
    try {
      const result = await fetcherRef.current();
      setData(result);
    } catch {
      /* offline / error — keep previous data */
    } finally {
      setLoading(false);
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

  return { data, setData, loading, refresh } as const;
}
