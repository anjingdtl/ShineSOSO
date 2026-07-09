import { useState } from 'react';
import type { SearchResult } from '../../types';
import { formatBytes, formatCount, formatRelativeTime } from '../../utils/format';

type CopyState = 'idle' | 'copied' | 'error';

async function copyToClipboard(text: string): Promise<boolean> {
    try {
        await navigator.clipboard.writeText(text);
        return true;
    } catch {
        return false;
    }
}

function openExternal(url: string, kind: string): void {
    // For magnet/torrent/direct, try the OS handler; for detail, open in new tab.
    const a = document.createElement('a');
    if (kind === 'detail') {
        a.target = '_blank';
        a.rel = 'noopener noreferrer';
    }
    a.href = url;
    a.click();
}

const KIND_LABEL: Record<string, string> = {
    magnet: '磁力',
    torrent: '种子',
    direct: '直链',
    detail: '详情',
};

export function ResultCard({ result }: { result: SearchResult }): JSX.Element {
    const primaryURL = result.magnetUrl ?? result.torrentUrl ?? result.directUrl ?? result.detailUrl ?? '';
    const kind = result.magnetUrl ? 'magnet'
        : result.torrentUrl ? 'torrent'
        : result.directUrl ? 'direct'
        : result.detailUrl ? 'detail'
        : '';
    const [copyState, setCopyState] = useState<CopyState>('idle');

    const onCopy = async () => {
        if (!primaryURL) return;
        const ok = await copyToClipboard(primaryURL);
        setCopyState(ok ? 'copied' : 'error');
        setTimeout(() => setCopyState('idle'), 1500);
    };

    return (
        <article className="result-card" data-kind={kind}>
            <header className="result-card-header">
                <h3 className="result-card-title">{result.title}</h3>
                <span className={`result-card-kind kind-${kind}`}>{KIND_LABEL[kind] ?? '?'}</span>
            </header>
            <div className="result-card-meta">
                <span>{formatBytes(result.sizeBytes)}</span>
                <span>· {formatCount(result.seeders ?? null)} 做种</span>
                {result.downloads != null && <span>· {formatCount(result.downloads)} 下载</span>}
                <span>· {formatRelativeTime(result.publishedAt)}</span>
            </div>
            <div className="result-card-source">
                来源：{result.indexerName}
            </div>
            <div className="result-card-actions">
                <button
                    className="btn"
                    type="button"
                    onClick={onCopy}
                    disabled={!primaryURL}
                    aria-label="复制下载链接"
                >
                    {copyState === 'copied' ? '已复制' : copyState === 'error' ? '复制失败' : '复制链接'}
                </button>
                <button
                    className="btn"
                    type="button"
                    onClick={() => openExternal(primaryURL, kind)}
                    disabled={!primaryURL}
                    aria-label="打开链接"
                >
                    打开
                </button>
                {result.detailUrl && result.detailUrl !== primaryURL && (
                    <button
                        className="btn btn-link"
                        type="button"
                        onClick={() => openExternal(result.detailUrl!, 'detail')}
                    >
                        详情
                    </button>
                )}
            </div>
        </article>
    );
}
