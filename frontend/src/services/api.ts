// Thin fetch wrapper that points at the same origin as the current page.
// In production the frontend is served by the Go binary on 127.0.0.1:0;
// in dev the Vite proxy forwards /api to the Go backend on the port
// declared in $EASYSEARCH_DATA_DIR/.port.

import type { SystemStatus } from '../types';

export class ApiError extends Error {
    constructor(
        public readonly status: number,
        public readonly code: string,
        message: string,
    ) {
        super(message);
        this.name = 'ApiError';
    }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
    const res = await fetch(path, {
        ...init,
        headers: {
            Accept: 'application/json',
            ...(init?.headers ?? {}),
        },
    });
    if (!res.ok) {
        let code = 'INTERNAL_ERROR';
        let message = res.statusText;
        try {
            const body = (await res.json()) as { error?: { code: string; message: string } };
            if (body.error) {
                code = body.error.code;
                message = body.error.message;
            }
        } catch {
            // non-JSON body; use the statusText fallback
        }
        throw new ApiError(res.status, code, message);
    }
    return (await res.json()) as T;
}

export const api = {
    getSystemStatus(): Promise<SystemStatus> {
        return request<SystemStatus>('/api/v1/system/status');
    },
};
