import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import { TodayRevenueHeader } from './TodayRevenueHeader';
import { renderWithProviders } from '@/test/test-utils';

const totals = {
  bookings: 120000,
  bar_sales: 45000,
  other_income: 15000,
  expenses: 8000,
  total_income: 180000,
  by_method: { cash: 100000, mercadopago: 80000 },
};

describe('TodayRevenueHeader', () => {
  it('renders as "Ingresos de hoy" — renamed from "Control de caja", which this card never was', () => {
    renderWithProviders(<TodayRevenueHeader totals={totals} />);
    expect(screen.getByText('Ingresos de hoy')).toBeInTheDocument();
    expect(screen.queryByText('Control de caja')).not.toBeInTheDocument();
  });

  it('totals bookings plus the till income, and shows where it came from', () => {
    renderWithProviders(<TodayRevenueHeader totals={totals} />);
    expect(screen.getByText('$1.800')).toBeInTheDocument();
    expect(screen.getByText('Turnos', { exact: false })).toHaveTextContent('$1.200');
    expect(screen.getByText('Bar', { exact: false })).toHaveTextContent('$450');
    expect(screen.getByText('Otros', { exact: false })).toHaveTextContent('$150');
  });
});
