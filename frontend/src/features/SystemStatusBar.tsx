import { useQuery } from '@tanstack/react-query';
import { api } from '../services/api';

export function SystemStatusBar(): JSX.Element {
    const { data, isLoading, isError } = useQuery({
        queryKey: ['system-status'],
        queryFn: api.getSystemStatus,
        refetchInterval: 5_000,
    });

    if (isLoading) {
        return <span className="status-pill status-pill-loading">…</span>;
    }
    if (isError || !data) {
        return <span className="status-pill status-pill-error">离线</span>;
    }
    return (
        <span className="status-pill" title={`started ${data.startedAt}`}>
            v{data.version} · 已运行 {Math.max(0, Math.floor(data.uptimeMs / 1000))}s
        </span>
    );
}
