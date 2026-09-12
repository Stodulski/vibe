import { describe, it, expect, vi } from 'vitest';
import { useCallback, useState } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ClientGrid } from './ClientGrid';
import type { Client } from '@/shared/types/api.types';

// `ClientCard`'s render body always calls `attendancePct` — spying on it is a
// cheap, reliable way to tell whether `memo` actually bailed out on a
// re-render (a bailed-out component never calls its own body again),
// without depending on `Profiler` timing quirks.
vi.mock('./client-card/attendance', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./client-card/attendance')>();
  return { ...actual, attendancePct: vi.fn(actual.attendancePct) };
});

function makeClient(overrides: Partial<Client> = {}): Client {
  return {
    id: 'cl1',
    complex_id: 'c1',
    first_name: 'Juan',
    last_name: 'Perez',
    phone: '+541155550000',
    is_blocked: false,
    total_bookings: 10,
    no_shows: 2,
    created_at: '2026-03-14T10:00:00Z',
    updated_at: '2026-03-14T10:00:00Z',
    ...overrides,
  };
}

/**
 * `onSelectClient`/`onBlockClient` are `useCallback`-stable across re-renders
 * (as `useClientActions`'s real handlers are) and `client` never changes
 * identity — the only thing that changes is the unrelated `tick` state, which
 * forces this harness (and therefore `ClientGrid`, which isn't memoized) to
 * re-render, the same way any parent state change would.
 */
function Harness({ client }: { client: Client }) {
  const [, setTick] = useState(0);
  const onSelectClient = useCallback(() => {
    /* no-op: only identity is under test */
  }, []);
  const onBlockClient = useCallback(() => {
    /* no-op: only identity is under test */
  }, []);

  return (
    <>
      <button
        onClick={() => {
          setTick((n) => n + 1);
        }}
      >
        force parent re-render
      </button>
      <ClientGrid clients={[client]} onSelectClient={onSelectClient} onBlockClient={onBlockClient} />
    </>
  );
}

describe('ClientGrid — ClientCard stays memoized once the grid stops wrapping onSelect in a new closure (02-bookings-clients.md M8)', () => {
  it('does not re-render an unchanged card when the grid re-renders for an unrelated reason', async () => {
    const { attendancePct } = await import('./client-card/attendance');
    const user = userEvent.setup();
    render(<Harness client={makeClient()} />);

    expect(attendancePct).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole('button', { name: 'force parent re-render' }));

    // Still 1: every prop `ClientCard` receives (`client`, `onSelect`,
    // `onBlock`) kept the same identity, so `memo` bailed out and the card's
    // body never ran again.
    expect(attendancePct).toHaveBeenCalledTimes(1);
  });
});
