import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent, { type UserEvent } from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import CashPage from './CashPage';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { makeCashSession } from '@/test/factories';

// Deliberately NOT mocking the cash hooks (`useCashSession`,
// `useCashSessionDetail`, `useCashSessions`) here, unlike `CashPage.test.tsx`
// — this is the page-level regression test for the T3 review finding "close
// result unmount": it needs the real `CloseCashSessionDialog`, the real
// `useCashPage` state machine, and a real `QueryClient` so the close
// mutation's own cache invalidation actually flips `useCashSession` from
// open to closed mid-dialog, the exact sequence the bug depended on.
vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({ useSelectedComplex: vi.fn() }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const summary = (expected: number) => ({
  opening_cash: 1000000,
  expected_cash: expected,
  cash_manual_refunds: 0,
  movement_totals: [],
  booking_payments: [],
  manual_refunds: [],
});

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <CashPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

/** Session A starts open; closing it (route below) flips `current` to 'closed'; opening again (route below) flips to session B. */
function mockCloseThenReopenApi() {
  const sessionA = makeCashSession({ id: 's1' });
  const closedA = makeCashSession({
    id: 's1',
    closed_at: '2026-01-01T12:00:00Z',
    counted_cash: 950000,
    expected_cash: 1000000,
    difference: -50000,
  });
  const sessionB = makeCashSession({ id: 's2' });
  let current: 'open-a' | 'closed' | 'open-b' = 'open-a';

  server.use(
    http.get('*/complexes/:complexId/cash-session', () => {
      if (current === 'open-a') return HttpResponse.json({ cash_session: sessionA, summary: summary(1000000) });
      if (current === 'open-b') return HttpResponse.json({ cash_session: sessionB, summary: summary(1000000) });
      return HttpResponse.json(
        { type: 'https://vibe.com.ar/problems/not-found', title: 'Not Found', status: 404 },
        { status: 404 },
      );
    }),
    http.get('*/complexes/:complexId/cash-sessions/:sessionId', ({ params }) => {
      const session = params.sessionId === 's2' ? sessionB : sessionA;
      return HttpResponse.json({ cash_session: session, summary: summary(1000000), movements: [] });
    }),
    http.get('*/complexes/:complexId/cash-sessions', () =>
      HttpResponse.json({ cash_sessions: [], metadata: { has_more: false } }),
    ),
    http.post('*/complexes/:complexId/cash-sessions/:sessionId/close', () => {
      current = 'closed';
      return HttpResponse.json({ cash_session: closedA });
    }),
    http.post('*/complexes/:complexId/cash-sessions', () => {
      current = 'open-b';
      return HttpResponse.json({ cash_session: sessionB }, { status: 201 });
    }),
  );
}

/** Closes session A with a counted $9.500 (a $500 shortfall) and returns the still-open dialog showing the committed result. */
async function closeSessionAWithShortfall(user: UserEvent) {
  await user.click(screen.getByRole('button', { name: 'Cerrar caja' }));
  const closeDialog = screen.getByRole('dialog');
  await user.type(within(closeDialog).getByLabelText('Efectivo contado'), '9500');
  await user.click(within(closeDialog).getByRole('button', { name: 'Cerrar caja' }));

  // The committed result, with the right difference, even once the close's
  // own cache invalidation resolves `useCashSession` to closed and
  // `CashPageBody` swaps to `ClosedCashView` underneath — the dialog must
  // not be unmounted by that swap (T3 review).
  await waitFor(() => {
    expect(within(closeDialog).getByText('Faltante: $500')).toBeInTheDocument();
  });
  await waitFor(() => {
    expect(screen.getByText('La caja está cerrada')).toBeInTheDocument();
  });
  expect(closeDialog).toBeVisible();
  return closeDialog;
}

async function dismissClosedResult(user: UserEvent, closeDialog: HTMLElement) {
  const doneButton = within(closeDialog)
    .getAllByRole('button', { name: 'Cerrar' })
    .find((b) => b.getAttribute('data-slot') !== 'dialog-close');
  if (!doneButton) throw new Error('done button not found');
  await user.click(doneButton);
  await waitFor(() => {
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
}

async function openSessionB(user: UserEvent) {
  await user.click(screen.getByRole('button', { name: 'Abrir caja' }));
  const openDialog = screen.getByRole('dialog');
  await user.type(within(openDialog).getByLabelText('Monto inicial'), '5000');
  await user.click(within(openDialog).getByRole('button', { name: 'Abrir caja' }));

  await waitFor(() => {
    expect(screen.getByText('Efectivo esperado')).toBeInTheDocument();
  });
}

describe('CashPage — close flow (real dialog, mocked API)', () => {
  it('shows the committed close result, lets the person dismiss it, and never reopens the close dialog on the next session', async () => {
    const user = userEvent.setup();
    vi.mocked(useSelectedComplex).mockReturnValue({
      complex: { id: 'c1' },
      selectedComplexId: 'c1',
    } as unknown as ReturnType<typeof useSelectedComplex>);
    mockCloseThenReopenApi();

    renderPage();
    await screen.findByText('Efectivo esperado');

    const closeDialog = await closeSessionAWithShortfall(user);
    await dismissClosedResult(user, closeDialog);
    await openSessionB(user);

    // The close dialog from session A must never leak open over session B.
    expect(screen.queryByRole('heading', { name: 'Cerrar caja' })).not.toBeInTheDocument();
  });
});
