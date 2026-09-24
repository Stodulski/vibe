import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ReportHeadline } from './ReportHeadline';
import type { MonthlyReportTotals } from '@/shared/types/api.types';

function makeTotals(overrides: Partial<MonthlyReportTotals> = {}): MonthlyReportTotals {
  return {
    count: 0,
    total: 0,
    refunded: 0,
    net: 0,
    service_fees: 0,
    ...overrides,
  };
}

describe('ReportHeadline', () => {
  it('shows the net amount smaller on mobile and at its full size from sm up', () => {
    render(<ReportHeadline totals={makeTotals({ net: 999999999 })} previous={makeTotals()} />);

    const amount = screen.getByText('$9.999.999,99');
    expect(amount).toHaveClass('text-xl', 'sm:text-2xl', 'tabular-nums');
  });
});
