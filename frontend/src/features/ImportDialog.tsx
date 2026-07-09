import { useState } from 'react';
import type { ChangeEvent } from 'react';
import { api, ApiError } from '../services/api';
import type { ImportResponse } from '../types';

type Status =
    | { kind: 'idle' }
    | { kind: 'loading' }
    | { kind: 'done'; data: ImportResponse }
    | { kind: 'error'; message: string };

interface Props {
    onClose: () => void;
    onInstalled: () => void;
}

export function ImportDialog({ onClose, onInstalled }: Props): JSX.Element {
    const [yaml, setYaml] = useState('');
    const [filename, setFilename] = useState('uploaded.yml');
    const [status, setStatus] = useState<Status>({ kind: 'idle' });
    const [installBaseUrl, setInstallBaseUrl] = useState('https://');

    const onFile = async (e: ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];
        if (!file) return;
        setFilename(file.name);
        setYaml(await file.text());
    };

    const submit = async () => {
        if (!yaml.trim()) return;
        setStatus({ kind: 'loading' });
        try {
            const res = await api.importDefinition(yaml, filename, true);
            setStatus({ kind: 'done', data: res });
        } catch (err) {
            const msg = err instanceof ApiError ? err.message : err instanceof Error ? err.message : String(err);
            setStatus({ kind: 'error', message: msg });
        }
    };

    const install = async () => {
        if (status.kind !== 'done' || !status.data.valid || !status.data.definition) return;
        try {
            await api.createIndexer({
                definitionId: status.data.definition.id,
                baseUrl: installBaseUrl.trim(),
                testBeforeEnable: false,
                enabled: true,
            });
            onInstalled();
            onClose();
        } catch (err) {
            const msg = err instanceof ApiError ? err.message : err instanceof Error ? err.message : String(err);
            setStatus({ kind: 'error', message: `已成功导入定义，但安装失败：${msg}` });
        }
    };

    return (
        <div className="modal-backdrop" role="dialog" aria-modal="true">
            <div className="modal">
                <header className="modal-header">
                    <h2>导入 YAML</h2>
                    <button type="button" className="btn-link" onClick={onClose}>关闭</button>
                </header>

                <div className="modal-body">
                    <label className="form-row">
                        <span>选择 .yml 文件</span>
                        <input type="file" accept=".yml,.yaml" onChange={onFile} />
                    </label>
                    <label className="form-row">
                        <span>或直接粘贴</span>
                        <textarea
                            rows={10}
                            value={yaml}
                            onChange={(e) => setYaml(e.target.value)}
                            spellCheck={false}
                            placeholder={'schema: 1\nid: example\nname: Example\n…'}
                        />
                    </label>

                    {status.kind === 'loading' && <p>校验中…</p>}
                    {status.kind === 'error' && (
                        <div className="notice notice-error">{status.message}</div>
                    )}
                    {status.kind === 'done' && (
                        <div className="import-decision">
                            {status.data.valid ? (
                                <>
                                    <p className="notice notice-success">YAML 校验通过。</p>
                                    {status.data.errors && status.data.errors.length > 0 && (
                                        <ul className="import-errors">
                                            {status.data.errors.map((e, i) => (
                                                <li key={i}><code>{e.code}</code>：{e.message}</li>
                                            ))}
                                        </ul>
                                    )}
                                    {status.data.test && (
                                        <div className={`notice ${status.data.test.ok ? 'notice-success' : 'notice-error'}`}>
                                            {status.data.test.ok
                                                ? `测试通过（${status.data.test.durationMs} ms${status.data.test.statusCode ? `，HTTP ${status.data.test.statusCode}` : ''}）`
                                                : `测试失败：${status.data.test.errorMessage ?? status.data.test.errorCode ?? 'unknown'}`}
                                        </div>
                                    )}
                                    <label className="form-row">
                                        <span>Base URL</span>
                                        <input
                                            type="url"
                                            value={installBaseUrl}
                                            onChange={(e) => setInstallBaseUrl(e.target.value)}
                                            placeholder="https://example.com"
                                            required
                                        />
                                    </label>
                                </>
                            ) : (
                                <>
                                    <div className="notice notice-error">校验未通过：</div>
                                    <ul className="import-errors">
                                        {(status.data.errors ?? []).map((e, i) => (
                                            <li key={i}><code>{e.code}</code>：{e.message}</li>
                                        ))}
                                    </ul>
                                </>
                            )}
                        </div>
                    )}
                </div>

                <footer className="modal-footer">
                    {status.kind !== 'done' || !status.data.valid ? (
                        <button type="button" className="btn btn-primary" onClick={submit} disabled={status.kind === 'loading' || !yaml.trim()}>
                            {status.kind === 'loading' ? '校验中…' : '校验'}
                        </button>
                    ) : (
                        <>
                            <button type="button" className="btn" onClick={onClose}>取消</button>
                            <button type="button" className="btn btn-primary" onClick={install}>
                                启用并保存
                            </button>
                        </>
                    )}
                </footer>
            </div>
        </div>
    );
}
