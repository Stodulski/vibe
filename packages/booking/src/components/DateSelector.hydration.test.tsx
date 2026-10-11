// The complex page is rendered by the server (UTC on Vercel) and hydrated in the
// browser (Argentina for the players). At 02:30 UTC it is still the previous day in
// Argentina, so the two must agree on "today" or hydration mismatches. This runs the
// server render in UTC, then hydrates the same markup in an Argentine browser.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { renderToString } from 'react-dom/server';
import { hydrateRoot, type Root } from 'react-dom/client';
import { DateSelector } from './DateSelector';
import { parseDateParam } from '../pages/complex-page/schema';

const SERVER_TZ = 'UTC';
const BROWSER_TZ = 'America/Argentina/Buenos_Aires';
const INSTANT = new Date('2026-10-11T02:30:00Z');

const originalTz = process.env.TZ;

function useZone(tz: string): void {
  process.env.TZ = tz;
}

const schedules: [] = [];

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] });
  vi.setSystemTime(INSTANT);
});

afterEach(() => {
  vi.useRealTimers();
  if (originalTz === undefined) delete process.env.TZ;
  else process.env.TZ = originalTz;
});

describe('DateSelector server render and hydration', () => {
  it('hydrates without a mismatch when the server runs in UTC and the browser in Argentina', () => {
    useZone(SERVER_TZ);
    const serverHtml = renderToString(
      <DateSelector selectedDate={parseDateParam(null)} onDateSelect={vi.fn()} schedules={schedules} />,
    );

    useZone(BROWSER_TZ);
    const container = document.createElement('div');
    container.innerHTML = serverHtml;
    document.body.appendChild(container);
    const recoverable: unknown[] = [];
    const consoleErrors = vi.spyOn(console, 'error').mockImplementation(() => undefined);

    let root: Root | undefined;
    act(() => {
      root = hydrateRoot(
        container,
        <DateSelector selectedDate={parseDateParam(null)} onDateSelect={vi.fn()} schedules={schedules} />,
        { onRecoverableError: (error) => recoverable.push(error) },
      );
    });

    expect(recoverable).toEqual([]);
    expect(consoleErrors).not.toHaveBeenCalled();
    expect(container.textContent).toContain('10 de oct');

    root?.unmount();
    container.remove();
    consoleErrors.mockRestore();
  });
});
