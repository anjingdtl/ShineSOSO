export function SearchPage(): JSX.Element {
    return (
        <section className="page-empty">
            <h1>搜索</h1>
            <p className="empty-state">尚未添加索引器</p>
            <p className="empty-hint">添加至少一个公开索引器后即可开始搜索。</p>
        </section>
    );
}
