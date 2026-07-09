import type { SearchState } from './useSearchStream';

export function SearchStatus({ state }: { state: SearchState }): JSX.Element | null {
    const statuses = Object.values(state.indexers);
    if (statuses.length === 0 && state.status === 'idle') return null;

    const counts = {
        success: statuses.filter((s) => s.status === 'success').length,
        running: statuses.filter((s) => s.status === 'running').length,
        empty: statuses.filter((s) => s.status === 'empty').length,
        timeout: statuses.filter((s) => s.status === 'timeout').length,
        error: statuses.filter((s) => s.status === 'error').length,
    };

    return (
        <div className="search-status" role="status" aria-live="polite">
            <div className="search-status-summary">
                {state.status === 'streaming' && <span>搜索中…</span>}
                {state.status === 'completed' && <span>完成 · 总耗时 {state.totalMs} ms</span>}
                {state.status === 'cancelled' && <span>已取消</span>}
                {state.status === 'error' && <span>错误：{state.errorMessage ?? state.errorCode}</span>}
            </div>
            <div className="search-status-counts">
                <span className="badge badge-success">{counts.success} 成功</span>
                <span className="badge badge-running">{counts.running} 搜索中</span>
                <span className="badge badge-empty">{counts.empty} 空结果</span>
                <span className="badge badge-timeout">{counts.timeout} 超时</span>
                <span className="badge badge-error">{counts.error} 失败</span>
            </div>
            <div className="search-status-totals">
                原始结果 {state.rawCount} · 合并后 {state.mergedCount}
            </div>
        </div>
    );
}
