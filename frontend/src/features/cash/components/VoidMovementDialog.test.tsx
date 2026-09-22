import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeCashMovement } from '@/test/factories';
import { VoidMovementDialog } from './VoidMovementDialog';
import { useVoidCashMovement } from '../hooks/useVoidCashMovement';

vi.mock('../hooks/useVoidCashMovement', () => ({ useVoidCashMovement: vi.fn() }));

describe('VoidMovementDialog', () => {
  const mutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useVoidCashMovement).mockReturnValue({ mutate, isPending: false } as unknown as ReturnType<
      typeof useVoidCashMovement
    >);
  });

  it('renders nothing when no movement is targeted', () => {
    render(<VoidMovementDialog movement={null} onClose={vi.fn()} complexId="c1" sessionId="s1" />);
    expect(screen.queryByText('Anular movimiento')).not.toBeInTheDocument();
  });

  it('confirms with the movement id and an optional note', async () => {
    const user = userEvent.setup();
    const movement = makeCashMovement({ id: 'm1' });
    render(<VoidMovementDialog movement={movement} onClose={vi.fn()} complexId="c1" sessionId="s1" />);

    await user.type(screen.getByLabelText(/Nota/), 'error de tipeo');
    await user.click(screen.getByRole('button', { name: 'Anular' }));

    expect(mutate).toHaveBeenCalledWith({ movementId: 'm1', note: 'error de tipeo' }, expect.anything());
  });

  it('cancels without calling mutate', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<VoidMovementDialog movement={makeCashMovement()} onClose={onClose} complexId="c1" sessionId="s1" />);

    await user.click(screen.getByRole('button', { name: 'Cancelar' }));
    expect(mutate).not.toHaveBeenCalled();
  });
});
