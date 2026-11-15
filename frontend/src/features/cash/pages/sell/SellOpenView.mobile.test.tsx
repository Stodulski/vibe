import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SellOpenView } from './SellOpenView';
import type { useSellPage } from './useSellPage';

const t = ES_AR;

vi.mock('@/shared/hooks/useMediaQuery', () => ({ useMediaQuery: () => false }));
vi.mock('../../components/sell/SellProductGrid', () => ({ SellProductGrid: () => null }));
vi.mock('../../components/sell/SellBookingBalances', () => ({ SellBookingBalances: () => null }));
vi.mock('../../components/sell/SellCart', () => ({
  SellCart: ({ placement }: { placement?: string }) => <p>cart:{placement}</p>,
}));
vi.mock('../../components/sell/SellMobileCartBar', () => ({ SellMobileCartBar: () => null }));
vi.mock('../../components/sell/SalesSection', () => ({ SalesSection: () => null }));

function makeState(overrides: Partial<ReturnType<typeof useSellPage>> = {}): ReturnType<typeof useSellPage> {
  return {
    productsQuery: { isError: false, refetch: vi.fn() },
    visibleProducts: [],
    categories: [],
    lines: [],
    balanceLines: [],
    balanceDue: { bookings: [], isLoading: false, isError: false, refetch: vi.fn() },
    mobileCartOpen: true,
    setMobileCartOpen: vi.fn(),
    itemCount: 0,
    total: 0,
    ...overrides,
  } as unknown as ReturnType<typeof useSellPage>;
}

describe('SellOpenView — mobile cart sheet', () => {
  it('shows the cart in sheet placement under the shared drawer header, closable from its X', async () => {
    const user = userEvent.setup();
    const setMobileCartOpen = vi.fn();
    render(<SellOpenView complexId="c1" sessionId="s1" state={makeState({ setMobileCartOpen })} />);

    expect(await screen.findByText('cart:sheet')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: t.cash.cartTitle })).toBeInTheDocument();
    // One close control: the header's, not the sheet's own absolutely positioned one.
    await user.click(screen.getByRole('button', { name: t.common.close }));
    expect(setMobileCartOpen).toHaveBeenCalledWith(false);
  });
});
