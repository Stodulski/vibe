import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeSale } from '@/test/factories';
import { VoidSaleDialog } from './VoidSaleDialog';
import { useVoidSale } from '../../hooks/useVoidSale';

vi.mock('../../hooks/useVoidSale', () => ({ useVoidSale: vi.fn() }));

describe('VoidSaleDialog', () => {
  const mutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useVoidSale).mockReturnValue({ mutate, isPending: false } as unknown as ReturnType<typeof useVoidSale>);
  });

  it('renders nothing when no sale is targeted', () => {
    render(<VoidSaleDialog sale={null} onClose={vi.fn()} complexId="c1" sessionId="s1" />);
    expect(screen.queryByText('Anular venta')).not.toBeInTheDocument();
  });

  it('confirms with the sale id and an optional note', async () => {
    const user = userEvent.setup();
    const sale = makeSale({ id: 'sale1' });
    render(<VoidSaleDialog sale={sale} onClose={vi.fn()} complexId="c1" sessionId="s1" />);

    await user.type(screen.getByLabelText(/Nota/), 'producto roto');
    await user.click(screen.getByRole('button', { name: 'Anular' }));

    expect(mutate).toHaveBeenCalledWith({ saleId: 'sale1', note: 'producto roto' }, expect.anything());
  });

  it('cancels without calling mutate', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<VoidSaleDialog sale={makeSale()} onClose={onClose} complexId="c1" sessionId="s1" />);

    await user.click(screen.getByRole('button', { name: 'Cancelar' }));
    expect(mutate).not.toHaveBeenCalled();
  });
});
