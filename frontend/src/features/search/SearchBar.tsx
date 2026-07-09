import { useState, type FormEvent } from 'react';

export type SearchBarProps = {
    disabled?: boolean;
    onSubmit: (req: { keyword: string; category: string; sort: string }) => void;
    initialKeyword?: string;
    initialCategory?: string;
    initialSort?: string;
};

const CATEGORIES: Array<[string, string]> = [
    ['all', '全部'],
    ['movie', '电影'],
    ['tv', '剧集'],
    ['music', '音乐'],
    ['game', '游戏'],
    ['software', '软件'],
    ['book', '图书'],
    ['anime', '动漫'],
    ['other', '其他'],
];

const SORTS: Array<[string, string]> = [
    ['relevance', '综合排序'],
    ['seeders', '做种数优先'],
    ['publishedAt', '最新发布'],
    ['sizeDesc', '文件从大到小'],
    ['sizeAsc', '文件从小到大'],
];

export function SearchBar(props: SearchBarProps): JSX.Element {
    const [keyword, setKeyword] = useState(props.initialKeyword ?? '');
    const [category, setCategory] = useState(props.initialCategory ?? 'all');
    const [sort, setSort] = useState(props.initialSort ?? 'relevance');

    const onFormSubmit = (e: FormEvent) => {
        e.preventDefault();
        const trimmed = keyword.trim();
        if (!trimmed) return;
        props.onSubmit({ keyword: trimmed, category, sort });
    };

    return (
        <form className="search-bar" onSubmit={onFormSubmit} role="search">
            <div className="search-bar-row">
                <input
                    className="search-input"
                    type="text"
                    autoFocus
                    placeholder="输入关键词"
                    value={keyword}
                    onChange={(e) => setKeyword(e.target.value)}
                    maxLength={200}
                    disabled={props.disabled}
                    aria-label="搜索关键词"
                />
                <button className="btn btn-primary" type="submit" disabled={props.disabled || !keyword.trim()}>
                    搜索
                </button>
            </div>
            <div className="search-bar-row">
                <label className="search-filter">
                    分类：
                    <select value={category} onChange={(e) => setCategory(e.target.value)} disabled={props.disabled}>
                        {CATEGORIES.map(([v, label]) => (
                            <option key={v} value={v}>{label}</option>
                        ))}
                    </select>
                </label>
                <label className="search-filter">
                    排序：
                    <select value={sort} onChange={(e) => setSort(e.target.value)} disabled={props.disabled}>
                        {SORTS.map(([v, label]) => (
                            <option key={v} value={v}>{label}</option>
                        ))}
                    </select>
                </label>
            </div>
        </form>
    );
}
