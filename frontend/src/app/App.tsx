import { useEffect } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from 'react-router-dom';
import { router } from './router';

const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            staleTime: 30_000,
            refetchOnWindowFocus: false,
        },
    },
});

export function App(): JSX.Element {
    useEffect(() => {
        const heartbeat = () => { void fetch('/api/v1/system/heartbeat', { method: 'POST', keepalive: true }); };
        heartbeat();
        const timer = window.setInterval(heartbeat, 5_000);
        return () => window.clearInterval(timer);
    }, []);

    return (
        <QueryClientProvider client={queryClient}>
            <RouterProvider router={router} />
        </QueryClientProvider>
    );
}
