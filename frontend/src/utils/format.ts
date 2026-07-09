// Human-friendly formatters used across the UI.

export function formatBytes(bytes: number | undefined | null): string {
    if (bytes === undefined || bytes === null) return '—';
    if (bytes < 1024) return `${bytes} B`;
    const units = ['KB', 'MB', 'GB', 'TB'];
    let v = bytes / 1024;
    let i = 0;
    while (v >= 1024 && i < units.length - 1) {
        v /= 1024;
        i++;
    }
    return `${v.toFixed(v < 10 ? 1 : 0)} ${units[i]}`;
}

export function formatRelativeTime(iso: string | undefined | null): string {
    if (!iso) return '—';
    const t = new Date(iso).getTime();
    if (Number.isNaN(t)) return iso;
    const diff = Date.now() - t;
    const sec = Math.round(diff / 1000);
    if (sec < 60) return `${sec} 秒前`;
    const min = Math.round(sec / 60);
    if (min < 60) return `${min} 分钟前`;
    const hr = Math.round(min / 60);
    if (hr < 24) return `${hr} 小时前`;
    const day = Math.round(hr / 24);
    if (day < 30) return `${day} 天前`;
    return new Date(iso).toLocaleDateString('zh-CN');
}

export function formatCount(n: number | undefined | null): string {
    if (n === undefined || n === null) return '—';
    return n.toLocaleString('zh-CN');
}
