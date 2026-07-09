import { describe, it, expect } from 'vitest';
import { formatBytes, formatCount, formatRelativeTime } from '../../src/utils/format';

describe('formatBytes edge cases', () => {
    it('returns em-dash for null/undefined/0', () => {
        expect(formatBytes(undefined)).toBe('—');
        expect(formatBytes(null)).toBe('—');
        expect(formatBytes(0)).toBe('0 B');
    });
    it('formats TB', () => {
        const tb = 1024 ** 4;
        expect(formatBytes(tb)).toMatch(/^1\.0 TB$/);
    });
    it('rounds MB > 10 without decimals', () => {
        expect(formatBytes(50 * 1024 * 1024)).toMatch(/^50 MB$/);
    });
    it('keeps one decimal for small MB', () => {
        expect(formatBytes(2.5 * 1024 * 1024)).toBe('2.5 MB');
    });
});

describe('formatCount edge cases', () => {
    it('zero', () => {
        expect(formatCount(0)).toBe('0');
    });
    it('negative', () => {
        expect(formatCount(-1)).toBe('-1');
    });
    it('large number', () => {
        expect(formatCount(1234567)).toBe('1,234,567');
    });
});

describe('formatRelativeTime boundaries', () => {
    it('returns em-dash for empty string', () => {
        expect(formatRelativeTime('')).toBe('—');
    });
    it('returns original string for unparseable', () => {
        expect(formatRelativeTime('not-a-date')).toBe('not-a-date');
    });
    it('uses minutes for older timestamps', () => {
        const t = new Date(Date.now() - 5 * 60 * 1000).toISOString();
        expect(formatRelativeTime(t)).toMatch(/分钟前/);
    });
    it('uses hours for very old timestamps', () => {
        const t = new Date(Date.now() - 5 * 60 * 60 * 1000).toISOString();
        expect(formatRelativeTime(t)).toMatch(/小时前/);
    });
    it('uses days for ancient timestamps', () => {
        const t = new Date(Date.now() - 5 * 24 * 60 * 60 * 1000).toISOString();
        expect(formatRelativeTime(t)).toMatch(/天前/);
    });
});