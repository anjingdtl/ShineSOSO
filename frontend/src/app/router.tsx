import { createBrowserRouter, Link, Outlet, type RouteObject } from 'react-router-dom';
import { SearchPage } from '../pages/SearchPage';
import { IndexerPage } from '../pages/IndexerPage';

function Shell(): JSX.Element {
    return (
        <div className="app-shell">
            <header className="app-header">
                <Link to="/" className="app-title">EasySearch</Link>
                <nav className="app-nav">
                    <Link to="/">搜索</Link>
                    <Link to="/indexers">索引器</Link>
                </nav>
            </header>
            <main className="app-main">
                <Outlet />
            </main>
        </div>
    );
}

const routes: RouteObject[] = [
    {
        path: '/',
        element: <Shell />,
        children: [
            { index: true, element: <SearchPage /> },
            { path: 'indexers', element: <IndexerPage /> },
        ],
    },
];

export const router = createBrowserRouter(routes);
