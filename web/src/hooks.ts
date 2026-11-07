import { useCallback, useEffect, useState } from 'react';
import { api, errorText } from './api';
export function useRemote<T>(path: string | null) {
    const [data, setData] = useState<T | null>(null), [loading, setLoading] = useState(!!path), [error, setError] = useState(''), [version, setVersion] = useState(0);
    const reload = useCallback(() => setVersion(v => v + 1), []);
    useEffect(() => { if (!path) {
        setData(null);
        setLoading(false);
        return;
    } const controller = new AbortController(); setLoading(true); setError(''); setData(null); api<T>(path, { signal: controller.signal }).then(value => { if (!controller.signal.aborted)
        setData(value); }).catch(e => { if (!controller.signal.aborted)
        setError(errorText(e)); }).finally(() => { if (!controller.signal.aborted)
        setLoading(false); }); return () => controller.abort(); }, [path, version]);
    return { data, loading, error, reload, setData };
}
