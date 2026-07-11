import { useCallback, useEffect, useState } from 'react';
import { ApiError, api } from '../services/api';
import type { DiscoveryCandidate, IndexerDefinition, IndexerTestResult, InstalledIndexer } from '../types';
import { ImportDialog } from '../features/ImportDialog';

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

    const [manualName, setManualName] = useState<string>('');
    const [manualBaseUrl, setManualBaseUrl] = useState<string>('https://');
    const [adding, setAdding] = useState(false);
    const [importOpen, setImportOpen] = useState(false);
    const [catalogQuery, setCatalogQuery] = useState('');
    const [quickAdding, setQuickAdding] = useState<string | null>(null);
    const [webQuery, setWebQuery] = useState('');
    const [webResults, setWebResults] = useState<DiscoveryCandidate[]>([]);
    const [webSearching, setWebSearching] = useState(false);
    const [probing, setProbing] = useState<string | null>(null);

    const refresh = useCallback(async () => {
        setLoading(true);
        try {
            const [list, cat] = await Promise.all([api.listIndexers(), api.listCatalog()]);
            setInstalled(list.items);
            setCatalog(cat.items);
        } catch (err) {
            setNotice({ kind: 'error', text: err instanceof Error ? err.message : String(err) });
        } finally {
            setLoading(false);
        }
    }, []);

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
                definitionId: 'example-torznab',
                name: manualName.trim(),
                baseUrl: manualBaseUrl.trim(),
                testBeforeEnable: true,
            });
            setManualName('');
            setManualBaseUrl('https://');
            setNotice({ kind: 'success', text: '已添加' });
            await refresh();
        } catch (err) {
            const msg = err instanceof ApiError ? err.message : err instanceof Error ? err.message : String(err);
            setNotice({ kind: 'error', text: `添加失败：${msg}` });
        } finally {
            setAdding(false);
        }
    }, [manualName, manualBaseUrl, refresh]);

    const onQuickAdd = useCallback(async (definition: IndexerDefinition) => {
        const baseUrl = definition.links?.[0] ?? 'https://example.com';
        setQuickAdding(definition.id); setNotice(null);
        try { await api.createIndexer({ definitionId: definition.id, baseUrl, testBeforeEnable: true }); setNotice({ kind: 'success', text: `已添加并测试「${definition.name}」` }); await refresh(); }
        catch (err) { setNotice({ kind: 'error', text: `添加失败：${err instanceof Error ? err.message : String(err)}` }); }
        finally { setQuickAdding(null); }
    }, [refresh]);
    const onWebSearch = useCallback(async (e: React.FormEvent) => { e.preventDefault(); setWebSearching(true); setNotice(null); try { const r = await api.discoverIndexers(webQuery); setWebResults(r.items); } catch (err) { setNotice({kind:'error',text:err instanceof Error?err.message:String(err)}); } finally { setWebSearching(false); } }, [webQuery]);
    const onProbe = useCallback(async (candidate: DiscoveryCandidate) => { setProbing(candidate.url); setNotice(null); try { const p = await api.probeIndexer(candidate.url); await api.createIndexer({definitionId:'example-torznab',name:candidate.name || '发现的 Torznab 索引器',baseUrl:p.baseUrl,testBeforeEnable:true}); setNotice({kind:'success',text:`已验证并添加「${candidate.name}」`}); await refresh(); } catch (err) { setNotice({kind:'error',text:`未能自动接入：${err instanceof Error?err.message:String(err)}`}); } finally { setProbing(null); } }, [refresh]);

    const discoverable = catalog.filter((d) => !d.id.startsWith('demo-') && !d.id.startsWith('example-'))
        .filter((d) => `${d.name} ${d.description ?? ''} ${d.language ?? ''} ${d.protocol}`.toLowerCase().includes(catalogQuery.trim().toLowerCase()));

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
                <p className="page-sub">从发现中心一键添加；高级用户可手动接入 Torznab 或导入 YAML。</p>
            </header>

            {notice && (
                <div className={`notice notice-${notice.kind}`}>{notice.text}</div>
            )}

            <section className="card discovery-center">
                <div><h2>发现中心</h2><p className="form-help">目录已随软件安装在本机；搜索、添加和测试均不依赖 Prowlarr 或云端服务。</p></div>
                <input className="discovery-search" value={catalogQuery} onChange={(e) => setCatalogQuery(e.target.value)} placeholder="搜索名称、语言或协议" aria-label="搜索本地索引器目录" />
                <ul className="discovery-results">{discoverable.map((definition) => <li key={definition.id}><div><strong>{definition.name}</strong><span>{[definition.description, definition.language, definition.protocol].filter(Boolean).join(' · ')}</span></div><button type="button" className="btn btn-primary" disabled={quickAdding !== null} onClick={() => void onQuickAdd(definition)}>{quickAdding === definition.id ? '测试中…' : '一键添加'}</button></li>)}</ul>
                {!loading && discoverable.length === 0 && <p className="empty-state">没有匹配项。你仍可使用下方手动添加或导入 YAML。</p>}
            </section>
            <section className="card discovery-center">
                <div><h2>联网发现索引器</h2><p className="form-help">关键词会发送到公开搜索服务。仅通过 Torznab 协议探测与测试的候选才会添加。</p></div>
                <form className="web-discovery-form" onSubmit={onWebSearch}><input className="discovery-search" value={webQuery} onChange={(e) => setWebQuery(e.target.value)} placeholder="例如：公开电影、中文电影" required minLength={2}/><button className="btn" disabled={webSearching}>{webSearching?'搜索中…':'搜索全网候选'}</button></form>
                {webResults.length > 0 && <ul className="discovery-results">{webResults.map((candidate)=><li key={candidate.url}><div><strong>{candidate.name || candidate.url}</strong><span>{candidate.summary || candidate.url}</span></div><button type="button" className="btn" disabled={probing!==null} onClick={() => void onProbe(candidate)}>{probing===candidate.url?'探测中…':'探测并添加'}</button></li>)}</ul>}
            </section>

            <details className="card manual-add">
                <summary>高级：手动添加 Torznab 接口或导入 YAML</summary>
                <p className="form-help">仅在你已有 Torznab 兼容服务地址时使用；普通用户请直接使用发现中心。</p>
                <form className="add-form" onSubmit={onAdd}>
                    <label className="form-row"><span>显示名称</span><input value={manualName} onChange={(e) => setManualName(e.target.value)} placeholder="例如：我的 Torznab 服务" required /></label>
                    <label className="form-row"><span>Torznab Base URL</span><input type="url" value={manualBaseUrl} onChange={(e) => setManualBaseUrl(e.target.value)} placeholder="https://example.com" required /></label>
                    <div className="manual-actions"><button type="submit" className="btn btn-primary" disabled={adding}>{adding ? '测试中…' : '添加并测试'}</button><button type="button" className="btn" onClick={() => setImportOpen(true)}>导入本地 YAML</button></div>
                </form>
            </details>

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
            {importOpen && (
                <ImportDialog
                    onClose={() => setImportOpen(false)}
                    onInstalled={() => {
                        setNotice({ kind: 'success', text: '已导入并安装' });
                        void refresh();
                    }}
                />
            )}
        </section>
    );
}
