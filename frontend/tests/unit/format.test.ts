import { describe, it, expect } from 'vitest';
import { formatBytes, formatCount, formatRelativeTime } from '../../src/utils/format';

describe('formatBytes', () => {
    it('returns em-dash for null/undefined', () => {
        expect(formatBytes(undefined)).toBe('—');
        expect(formatBytes(null)).toBe('—');
    });
    it('keeps bytes under 1 KiB as B', () => {
        expect(formatBytes(500)).toBe('500 B');
    });
    it('formats KiB / MiB / GiB', () => {
        expect(formatBytes(1024)).toBe('1.0 KB');
        expect(formatBytes(1024 * 1024)).toBe('1.0 MB');
        expect(formatBytes(8.4 * 1024 * 1024 * 1024)).toMatch(/^8\.\d GB$/);
    });
});

describe('formatCount', () => {
    it('em-dash for nullish', () => {
        expect(formatCount(undefined)).toBe('—');
        expect(formatCount(null)).toBe('—');
    });
    it('localizes integers', () => {
        expect(formatCount(1234)).toBe('1,234');
    });
});

describe('formatRelativeTime', () => {
    it('em-dash for nullish', () => {
        expect(formatRelativeTime(undefined)).toBe('—');
    });
    it('returns seconds for recent', () => {
        const now = new Date().toISOString();
        expect(formatRelativeTime(now)).toMatch(/秒前/);
    });
});
