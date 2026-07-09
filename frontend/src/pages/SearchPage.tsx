import { useQuery } from '@tanstack/react-query';
import { api } from '../services/api';
import { SearchBar } from '../features/search/SearchBar';
import { SearchStatus } from '../features/search/SearchStatus';
import { ResultList } from '../features/search/ResultList';
import { useSearchStream } from '../features/search/useSearchStream';

export function SearchPage(): JSX.Element {
    const { data: status } = useQuery({
        queryKey: ['system-status'],
        queryFn: api.getSystemStatus,
        refetchInterval: 5_000,
    });
    const { state, startSearch } = useSearchStream();
    const hasIndexers = (status?.installedIndexers ?? 0) > 0;

    return (
        <section className="search-page">
            <SearchBar
                onSubmit={(req) => void startSearch(req)}
                disabled={state.status === 'creating' || state.status === 'streaming'}
            />
            <SearchStatus state={state} />
            {state.status === 'idle' && !hasIndexers && (
                <div className="empty-state-block">
                    <p className="empty-state">尚未添加索引器</p>
                    <p className="empty-hint">添加至少一个公开索引器后即可开始搜索。</p>
                </div>
            )}
            {state.status === 'idle' && hasIndexers && (
                <div className="empty-state-block">
                    <p className="empty-hint">输入关键词并按 Enter 开始搜索。</p>
                </div>
            )}
            <ResultList results={state.results} />
        </section>
    );
}
