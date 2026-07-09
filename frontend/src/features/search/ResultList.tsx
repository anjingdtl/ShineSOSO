import type { SearchResult } from '../../types';
import { ResultCard } from './ResultCard';

export function ResultList({ results }: { results: SearchResult[] }): JSX.Element {
    if (results.length === 0) {
        return <p className="result-empty">暂无结果</p>;
    }
    return (
        <ul className="result-list">
            {results.map((r) => (
                <li key={r.id}>
                    <ResultCard result={r} />
                </li>
            ))}
        </ul>
    );
}
