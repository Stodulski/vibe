import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useTheme } from './useTheme';

describe('useTheme', () => {
  it('always returns dark theme', () => {
    const { result } = renderHook(() => useTheme());
    expect(result.current.theme).toBe('dark');
  });

  it('returns a no-op toggleTheme function', () => {
    const { result } = renderHook(() => useTheme());
    expect(typeof result.current.toggleTheme).toBe('function');
    result.current.toggleTheme();
    expect(result.current.theme).toBe('dark');
  });
});
