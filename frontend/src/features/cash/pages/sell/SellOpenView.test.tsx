import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SellOpenView } from './SellOpenView';
import type { useSellPage } from './useSellPage';

const t = ES_AR;

// Every child component is heavy (data grids, forms, a sales list query) and
// irrelevant to the notices region this test targets — stubbing them keeps
// this a focused test of `SellOpenView` itself, the same way `VoidSaleDialog`
// stubs `useVoidSale` rather than exercising the real mutation.
vi.mock('@/shared/hooks/useMediaQuery', () => ({ useMediaQuery: () => true }));
vi.mock('../../components/sell/SellProductGrid', () => ({ SellProductGrid: () => null }));
vi.mock('../../components/sell/SellCart', () => ({ SellCart: () => null }));
vi.mock('../../components/sell/SellMobileCartBar', () => ({ SellMobileCartBar: () => null }));
vi.mock('../../components/sell/SalesSection', () => ({ SalesSection: () => null }));

function makeState(overrides: Partial<ReturnType<typeof useSellPage>> = {}): ReturnType<typeof useSellPage> {
  return {
    complexId: 'c1',
    cashSession: {},
    productsQuery: { isError: false, refetch: vi.fn() },
    hasAnyProducts: true,
    visibleProducts: [],
    categories: [],
    search: '',
    setSearch: vi.fn(),
    category: null,
    setCategory: vi.fn(),
    addProduct: vi.fn(),
    lines: [],
    incrementCartLine: vi.fn(),
    decrementCartLine: vi.fn(),
    setCartLineQuantity: vi.fn(),
    removeCartLine: vi.fn(),
    total: 0,
    products: [],
    method: 'cash',
    setMethod: vi.fn(),
    note: '',
    setNote: vi.fn(),
    lineErrors: {},
    droppedStaleNotice: false,
    dismissDroppedStaleNotice: vi.fn(),
    cartLimitNotice: false,
    mobileCartOpen: false,
    setMobileCartOpen: vi.fn(),
    charge: vi.fn(),
    isCharging: false,
    saleResult: null,
    clearSaleResult: vi.fn(),
    ...overrides,
  } as unknown as ReturnType<typeof useSellPage>;
}

describe('SellOpenView — dropped stale lines notice', () => {
  it('dismisses the notice through its close control', async () => {
    const user = userEvent.setup();
    const dismiss = vi.fn();
    render(
      <SellOpenView
        complexId="c1"
        sessionId="s1"
        state={makeState({ droppedStaleNotice: true, dismissDroppedStaleNotice: dismiss })}
      />,
    );

    expect(screen.getByText(t.cash.cartDroppedStaleLines)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: t.common.close }));

    expect(dismiss).toHaveBeenCalledTimes(1);
  });

  it('renders nothing when there is no notice to show', () => {
    render(<SellOpenView complexId="c1" sessionId="s1" state={makeState({ droppedStaleNotice: false })} />);
    expect(screen.queryByText(t.cash.cartDroppedStaleLines)).not.toBeInTheDocument();
  });
});
