import { createBrowserRouter, Link, NavLink, Outlet, type RouteObject } from 'react-router-dom';
import { SearchPage } from '../pages/SearchPage';
import { IndexerPage } from '../pages/IndexerPage';
import { SystemStatusBar } from '../features/SystemStatusBar';

function Shell(): JSX.Element {
    return (
        <div className="app-shell">
            <header className="app-header">
                <Link to="/" className="app-title">EasySearch</Link>
                <nav className="app-nav">
                    <NavLink
                        to="/"
                        end
                        className={({ isActive }) => (isActive ? 'active' : '')}
                    >
                        搜索
                    </NavLink>
                    <NavLink
                        to="/indexers"
                        className={({ isActive }) => (isActive ? 'active' : '')}
                    >
                        索引器
                    </NavLink>
                </nav>
                <div className="app-status">
                    <SystemStatusBar />
                </div>
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
