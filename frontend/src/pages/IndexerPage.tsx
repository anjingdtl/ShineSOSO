import { useCallback, useEffect, useState } from 'react';
import { ApiError, api } from '../services/api';
import type { IndexerDefinition, IndexerTestResult, InstalledIndexer } from '../types';

type Notice = { kind: 'success' | 'error'; text: string } | null;

function StatusBadge({ status }: { status: InstalledIndexer['status'] }): JSX.Element {
    const label = (() => {
        switch (status) {
            case 'healthy': return '正常';
            case 'degraded': return '降级';
            case 'unhealthy': return '异常';
            case 'disabled': return '已停用';
            default: return '未知';
        }
    })();
    return <span className={`badge badge-${status}`}>{label}</span>;
}

export function IndexerPage(): JSX.Element {
    const [installed, setInstalled] = useState<InstalledIndexer[]>([]);
    const [catalog, setCatalog] = useState<IndexerDefinition[]>([]);
    const [loading, setLoading] = useState(true);
    const [notice, setNotice] = useState<Notice>(null);
    const [testing, setTesting] = useState<Record<string, boolean>>({});

    const [newDefId, setNewDefId] = useState<string>('');
    const [newBaseUrl, setNewBaseUrl] = useState<string>('https://');
    const [adding, setAdding] = useState(false);

    const refresh = useCallback(async () => {
        setLoading(true);
        try {
            const [list, cat] = await Promise.all([api.listIndexers(), api.listCatalog()]);
            setInstalled(list.items);
            setCatalog(cat.items);
            if (!newDefId && cat.items.length > 0 && cat.items[0]) {
                setNewDefId(cat.items[0].id);
            }
        } catch (err) {
            setNotice({ kind: 'error', text: err instanceof Error ? err.message : String(err) });
        } finally {
            setLoading(false);
        }
    }, [newDefId]);

    useEffect(() => {
        void refresh();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const onAdd = useCallback(async (e: React.FormEvent) => {
        e.preventDefault();
        setAdding(true);
        setNotice(null);
        try {
            await api.createIndexer({
                definitionId: newDefId,
                baseUrl: newBaseUrl.trim(),
                testBeforeEnable: true,
            });
            setNewBaseUrl('https://');
            setNotice({ kind: 'success', text: '已添加' });
            await refresh();
        } catch (err) {
            const msg = err instanceof ApiError ? err.message : err instanceof Error ? err.message : String(err);
            setNotice({ kind: 'error', text: `添加失败：${msg}` });
        } finally {
            setAdding(false);
        }
    }, [newDefId, newBaseUrl, refresh]);

    const onToggle = useCallback(async (idx: InstalledIndexer) => {
        setNotice(null);
        try {
            await api.updateIndexer(idx.id, { enabled: !idx.enabled });
            await refresh();
        } catch (err) {
            setNotice({ kind: 'error', text: err instanceof Error ? err.message : String(err) });
        }
    }, [refresh]);

    const onDelete = useCallback(async (idx: InstalledIndexer) => {
        if (!confirm(`确认删除「${idx.name}」？`)) return;
        setNotice(null);
        try {
            await api.deleteIndexer(idx.id);
            setNotice({ kind: 'success', text: '已删除' });
            await refresh();
        } catch (err) {
            setNotice({ kind: 'error', text: err instanceof Error ? err.message : String(err) });
        }
    }, [refresh]);

    const onTest = useCallback(async (idx: InstalledIndexer) => {
        setTesting((s) => ({ ...s, [idx.id]: true }));
        setNotice(null);
        try {
            const res: IndexerTestResult = await api.testIndexer(idx.id);
            setNotice({
                kind: res.ok ? 'success' : 'error',
                text: res.ok
                    ? `测试通过（${res.durationMs} ms）`
                    : `测试失败：${res.errorMessage ?? res.errorCode ?? 'unknown'}`,
            });
            await refresh();
        } catch (err) {
            setNotice({ kind: 'error', text: err instanceof Error ? err.message : String(err) });
        } finally {
            setTesting((s) => ({ ...s, [idx.id]: false }));
        }
    }, [refresh]);

    return (
        <section className="page">
            <header className="page-header">
                <h1>索引器</h1>
                <p className="page-sub">从内置目录一键添加，或导入本地 YAML（Phase 5）。</p>
            </header>

            {notice && (
                <div className={`notice notice-${notice.kind}`}>{notice.text}</div>
            )}

            <form className="card add-form" onSubmit={onAdd}>
                <h2>添加</h2>
                <label className="form-row">
                    <span>定义</span>
                    <select
                        value={newDefId}
                        onChange={(e) => setNewDefId(e.target.value)}
                        required
                    >
                        {catalog.map((d) => (
                            <option key={d.id} value={d.id}>
                                {d.name}（{d.protocol}）
                            </option>
                        ))}
                    </select>
                </label>
                <label className="form-row">
                    <span>Base URL</span>
                    <input
                        type="url"
                        value={newBaseUrl}
                        onChange={(e) => setNewBaseUrl(e.target.value)}
                        placeholder="https://example.com"
                        required
                    />
                </label>
                <button type="submit" className="btn btn-primary" disabled={adding}>
                    {adding ? '添加中…' : '添加并测试'}
                </button>
            </form>

            <div className="card">
                <h2>已安装（{installed.length}）</h2>
                {loading ? (
                    <p>加载中…</p>
                ) : installed.length === 0 ? (
                    <p className="empty-state">尚无索引器。先从上方添加一个。</p>
                ) : (
                    <ul className="indexer-list">
                        {installed.map((idx) => (
                            <li key={idx.id} className={`indexer-row ${idx.enabled ? '' : 'is-disabled'}`}>
                                <div className="indexer-main">
                                    <div className="indexer-name">
                                        <strong>{idx.name}</strong>
                                        <StatusBadge status={idx.status} />
                                    </div>
                                    <div className="indexer-meta">
                                        <span>{idx.definitionId}</span>
                                        <span>·</span>
                                        <span>{idx.baseUrl}</span>
                                    </div>
                                    {idx.lastError && (
                                        <div className="indexer-error">最近错误：{idx.lastError}</div>
                                    )}
                                </div>
                                <div className="indexer-actions">
                                    <button
                                        type="button"
                                        className="btn"
                                        onClick={() => onTest(idx)}
                                        disabled={!!testing[idx.id]}
                                    >
                                        {testing[idx.id] ? '测试中…' : '测试'}
                                    </button>
                                    <button
                                        type="button"
                                        className="btn"
                                        onClick={() => onToggle(idx)}
                                    >
                                        {idx.enabled ? '停用' : '启用'}
                                    </button>
                                    <button
                                        type="button"
                                        className="btn btn-danger"
                                        onClick={() => onDelete(idx)}
                                    >
                                        删除
                                    </button>
                                </div>
                            </li>
                        ))}
                    </ul>
                )}
            </div>
        </section>
    );
}