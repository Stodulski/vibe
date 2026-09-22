import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeCashSession } from '@/test/factories';
import { CloseCashSessionDialog } from './CloseCashSessionDialog';
import { useCloseCashSession } from '../hooks/useCloseCashSession';
import type { CashSession } from '@/shared/types/api.types';

vi.mock('../hooks/useCloseCashSession', () => ({ useCloseCashSession: vi.fn() }));

const props = { open: true, onClose: vi.fn(), complexId: 'c1', sessionId: 's1', expectedCash: 500000 };

describe('CloseCashSessionDialog — live difference', () => {
  const mutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useCloseCashSession).mockReturnValue({ mutate, isPending: false } as unknown as ReturnType<
      typeof useCloseCashSession
    >);
  });

  it('shows the expected cash figure up front', () => {
    render(<CloseCashSessionDialog {...props} />);
    expect(screen.getByText('$5.000')).toBeInTheDocument();
  });

  it('shows no difference until an amount is typed', () => {
    render(<CloseCashSessionDialog {...props} />);
    expect(screen.queryByText('Diferencia')).not.toBeInTheDocument();
  });

  it('shows sobrante live as soon as counted exceeds expected', async () => {
    const user = userEvent.setup();
    render(<CloseCashSessionDialog {...props} />);
    await user.type(screen.getByLabelText('Efectivo contado'), '5500');
    expect(await screen.findByText(/Sobrante/)).toBeInTheDocument();
  });

  it('shows faltante live as soon as counted is under expected', async () => {
    const user = userEvent.setup();
    render(<CloseCashSessionDialog {...props} />);
    await user.type(screen.getByLabelText('Efectivo contado'), '4000');
    expect(await screen.findByText(/Faltante/)).toBeInTheDocument();
  });

  it('shows sin diferencia when counted matches expected exactly', async () => {
    const user = userEvent.setup();
    render(<CloseCashSessionDialog {...props} />);
    await user.type(screen.getByLabelText('Efectivo contado'), '5000');
    expect(await screen.findByText('Sin diferencia')).toBeInTheDocument();
  });

  it('rejects submitting with no counted amount typed', async () => {
    const user = userEvent.setup();
    render(<CloseCashSessionDialog {...props} />);
    await user.click(screen.getByRole('button', { name: 'Cerrar caja' }));
    expect(await screen.findByText('Ingresá un monto')).toBeInTheDocument();
    expect(mutate).not.toHaveBeenCalled();
  });
});

describe('CloseCashSessionDialog — closed result', () => {
  const mutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useCloseCashSession).mockReturnValue({ mutate, isPending: false } as unknown as ReturnType<
      typeof useCloseCashSession
    >);
  });

  it('shows the closed result and lets the user dismiss back to the closed state', async () => {
    const user = userEvent.setup();
    const closed = makeCashSession({
      closed_at: '2026-01-02T00:00:00Z',
      counted_cash: 550000,
      expected_cash: 500000,
      difference: 50000,
    });
    mutate.mockImplementation(
      (_vars: unknown, options: { onSuccess: (result: { cash_session: CashSession }) => void }) => {
        options.onSuccess({ cash_session: closed });
      },
    );
    render(<CloseCashSessionDialog {...props} />);

    await user.type(screen.getByLabelText('Efectivo contado'), '5500');
    await user.click(screen.getByRole('button', { name: 'Cerrar caja' }));

    await waitFor(() => {
      expect(screen.getByText(/Sobrante/)).toBeInTheDocument();
    });
    expect(screen.getByText('$5.500')).toBeInTheDocument();

    // Two "Cerrar" buttons exist here: the dialog's own sr-only close (✕) and
    // the result view's own submit — scoped to exclude the former.
    const closeButtons = screen.getAllByRole('button', { name: 'Cerrar' });
    const doneButton = closeButtons.find((b) => b.getAttribute('data-slot') !== 'dialog-close');
    if (!doneButton) throw new Error('done button not found');
    await user.click(doneButton);
    expect(props.onClose).toHaveBeenCalled();
  });
});
