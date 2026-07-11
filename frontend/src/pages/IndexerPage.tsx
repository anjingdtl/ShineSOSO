import { useCallback, useEffect, useState } from 'react';
import { ApiError, api } from '../services/api';
import type { IndexerDefinition, IndexerTestResult, InstalledIndexer, ProwlarrCandidate, ProwlarrStatus } from '../types';
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
    const [engineStatus, setEngineStatus] = useState<ProwlarrStatus>({ state: 'starting', message: '正在检查内置引擎…' });
    const [engineQuery, setEngineQuery] = useState('');
    const [engineResults, setEngineResults] = useState<ProwlarrCandidate[]>([]);
    const [engineSearching, setEngineSearching] = useState(false);
    const [engineAdding, setEngineAdding] = useState<string | null>(null);

    const refresh = useCallback(async () => {
        setLoading(true);
        try {
            const [list, cat, engine] = await Promise.all([api.listIndexers(), api.listCatalog(), api.getProwlarrStatus()]);
            setInstalled(list.items);
            setCatalog(cat.items);
            setEngineStatus(engine);
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
    const onEngineSearch = useCallback(async (e: React.FormEvent) => { e.preventDefault(); setEngineSearching(true); setNotice(null); try { const r = await api.discoverProwlarrIndexers(engineQuery); setEngineResults(r.items); } catch (err) { setNotice({kind:'error',text:err instanceof Error?err.message:String(err)}); } finally { setEngineSearching(false); } }, [engineQuery]);
    const onEngineAdd = useCallback(async (candidate: ProwlarrCandidate) => { setEngineAdding(candidate.id); setNotice(null); try { await api.addProwlarrIndexer(candidate.id); setNotice({kind:'success',text:`Prowlarr 已测试并添加「${candidate.name}」，现在可以直接搜索资源。`}); } catch (err) { setNotice({kind:'error',text:`未能添加：${err instanceof Error?err.message:String(err)}`}); } finally { setEngineAdding(null); } }, []);

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
                <div><h2>本地公开资料源</h2><p className="form-help">适合公开数字馆藏。电影、BT/Usenet 等索引器请使用下面的内置 Prowlarr 引擎。</p></div>
                <input className="discovery-search" value={catalogQuery} onChange={(e) => setCatalogQuery(e.target.value)} placeholder="搜索名称、语言或协议" aria-label="搜索本地索引器目录" />
                <ul className="discovery-results">{discoverable.map((definition) => <li key={definition.id}><div><strong>{definition.name}</strong><span>{[definition.description, definition.language, definition.protocol].filter(Boolean).join(' · ')}</span></div><button type="button" className="btn btn-primary" disabled={quickAdding !== null} onClick={() => void onQuickAdd(definition)}>{quickAdding === definition.id ? '测试中…' : '一键添加'}</button></li>)}</ul>
                {!loading && discoverable.length === 0 && <p className="empty-state">没有匹配项。你仍可使用下方手动添加或导入 YAML。</p>}
            </section>
            <section className="card discovery-center">
                <div><h2>电影与资源索引器（内置 Prowlarr）</h2><p className="form-help">{engineStatus.state === 'ready' ? `引擎已就绪${engineStatus.version ? `（${engineStatus.version}）` : ''}；只显示无需账号即可一键测试的公开索引器。` : engineStatus.message ?? '引擎正在启动，请稍候。'}</p></div>
                <form className="web-discovery-form" onSubmit={onEngineSearch}><input className="discovery-search" value={engineQuery} onChange={(e) => setEngineQuery(e.target.value)} placeholder="输入名称，例如：YTS、Nyaa、1337x" /><button className="btn" disabled={engineSearching || engineStatus.state !== 'ready'}>{engineSearching?'搜索中…':'搜索 Prowlarr 索引器'}</button></form>
                {engineStatus.state === 'starting' && <p className="form-help">首次启动需要初始化索引器定义，约需数十秒；请稍候后刷新或再次搜索。</p>}
                {engineResults.length > 0 && <ul className="discovery-results">{engineResults.map((candidate) => <li key={candidate.id}><div><strong>{candidate.name}</strong><span>{[candidate.privacy, candidate.protocol].filter(Boolean).join(' · ')}</span>{candidate.reason && <span>{candidate.reason}</span>}</div>{candidate.canQuickAdd ? <button type="button" className="btn btn-primary" disabled={engineAdding !== null} onClick={() => void onEngineAdd(candidate)}>{engineAdding === candidate.id ? 'Prowlarr 测试中…' : '一键测试并添加'}</button> : <span className="badge badge-empty">需要配置</span>}</li>)}</ul>}
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
