import { keepTogether } from '@/shared/lib/keepTogether';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeProduct } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { BalanceLine } from '../../lib/balanceCart';
import { SellCart } from './SellCart';

const t = ES_AR;

const balance: BalanceLine = {
  bookingId: 'b1',
  courtName: 'Cancha 2',
  startTime: '19:00',
  clientName: 'Juan Pérez',
  amount: 350000,
};

function renderCart(overrides: Partial<Parameters<typeof SellCart>[0]> = {}) {
  const props = {
    lines: [],
    products: [makeProduct({ id: 'p1', name: 'Agua', price: 100000 })],
    total: 350000,
    method: 'cash' as const,
    onMethodChange: vi.fn(),
    note: '',
    onNoteChange: vi.fn(),
    lineErrors: {},
    onIncrement: vi.fn(),
    onDecrement: vi.fn(),
    onRemove: vi.fn(),
    onCharge: vi.fn(),
    isCharging: false,
    cartLimitReached: false,
    balanceLines: [balance],
    onRemoveBalance: vi.fn(),
    balanceChangedNotice: false,
    ...overrides,
  };
  render(<SellCart {...props} />);
  return props;
}

describe('SellCart — booking balances', () => {
  it('lists the balance with its label and amount, and charges it with no quantity control', () => {
    renderCart();

    const row = screen.getByTestId('sell-balance-line');
    expect(within(row).getByText('Saldo del turno')).toBeInTheDocument();
    expect(within(row).getByText('Cancha 2 · 19:00 · Juan Pérez')).toBeInTheDocument();
    expect(within(row).getByText(formatPrice(350000))).toBeInTheDocument();
    expect(within(row).queryByRole('button', { name: /Sumar/ })).not.toBeInTheDocument();
  });

  it('lets the cashier remove a balance from the cart', async () => {
    const user = userEvent.setup();
    const { onRemoveBalance } = renderCart();

    await user.click(
      screen.getByRole('button', {
        name: `${t.cash.cartRemove}: Saldo turno · ${keepTogether('Cancha 2')} 19:00 · Juan Pérez`,
      }),
    );

    expect(onRemoveBalance).toHaveBeenCalledWith('b1');
  });

  it('allows checkout with only balances, with no product lines', () => {
    renderCart({ lines: [], products: [], total: 350000 });

    expect(
      screen.getByRole('button', { name: `${t.cash.cartChargeActionPrefix} ${formatPrice(350000)}` }),
    ).toBeEnabled();
  });

  it('shows the balance changed notice when the last charge was refused for a stale booking', () => {
    renderCart({ balanceChangedNotice: true });

    expect(screen.getByRole('alert')).toHaveTextContent('Este turno ya cambió. Actualizamos la lista.');
  });
});

describe('SellCart — checkout controls', () => {
  it('keeps the note hidden behind the "Agregar nota" toggle until asked for', async () => {
    renderCart();
    expect(screen.queryByLabelText(/^Nota/)).not.toBeInTheDocument();

    const toggle = screen.getByRole('button', { name: 'Agregar nota' });
    await userEvent.click(toggle);
    expect(toggle).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByLabelText(/^Nota/)).toBeInTheDocument();
  });

  it('opens with the note visible when the cart already carries one', () => {
    renderCart({ note: 'Paga el lunes' });
    expect(screen.getByLabelText(/^Nota/)).toHaveValue('Paga el lunes');
  });
});

describe('SellCart — placement', () => {
  const lines = [{ productId: 'p1', quantity: 2 }];

  it('in the sheet pins the total, method and "Cobrar" in the sheet footer, apart from the lines', () => {
    renderCart({ placement: 'sheet', lines, balanceLines: [] });

    const footer = document.querySelector('[data-slot="sheet-footer"]');
    expect(footer).not.toBeNull();
    expect(footer).toContainElement(screen.getByTestId('sell-cart-total'));
    expect(footer).toContainElement(
      screen.getByRole('button', { name: `${t.cash.cartChargeActionPrefix} ${formatPrice(350000)}` }),
    );
    expect(footer).not.toContainElement(screen.getByText('Agua'));
  });

  it('in the column keeps the footer inside the panel, with no sheet footer', () => {
    renderCart({ lines, balanceLines: [] });

    expect(document.querySelector('[data-slot="sheet-footer"]')).toBeNull();
    expect(screen.getByTestId('sell-cart')).toContainElement(screen.getByTestId('sell-cart-total'));
  });

  it('in the sheet has no footer while the cart is empty', () => {
    renderCart({ placement: 'sheet', lines: [], balanceLines: [] });

    expect(document.querySelector('[data-slot="sheet-footer"]')).toBeNull();
  });
});
