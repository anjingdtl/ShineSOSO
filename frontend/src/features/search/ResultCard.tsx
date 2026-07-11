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
    const kind = result.magnetUrl ? 'magnet' : result.torrentUrl ? 'torrent' : result.directUrl ? 'direct' : result.detailUrl ? 'detail' : '';
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
                {result.magnetUrl && <>
                    <button className="btn" type="button" onClick={onCopy} aria-label="复制磁力链接">{copyState === 'copied' ? '已复制' : copyState === 'error' ? '复制失败' : '复制磁力'}</button>
                    <button className="btn btn-primary" type="button" onClick={() => openExternal(result.magnetUrl!, 'magnet')} aria-label="打开磁力链接">打开磁力</button>
                </>}
                {result.torrentUrl && <button className="btn btn-primary" type="button" onClick={() => openExternal(result.torrentUrl!, 'torrent')} aria-label="下载种子文件">下载种子</button>}
                {!result.magnetUrl && !result.torrentUrl && result.directUrl && <>
                    <button className="btn" type="button" onClick={onCopy} aria-label="复制直链">{copyState === 'copied' ? '链接已复制' : copyState === 'error' ? '复制失败' : '复制直链'}</button>
                    <button className="btn btn-primary" type="button" onClick={() => openExternal(result.directUrl!, 'direct')} aria-label="打开直链">打开直链</button>
                </>}
                {!primaryURL && <span className="form-help">该索引器仅返回了元数据，未提供下载链接。</span>}
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
