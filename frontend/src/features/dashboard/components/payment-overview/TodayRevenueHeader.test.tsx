import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import { TodayRevenueHeader } from './TodayRevenueHeader';
import { renderWithProviders } from '@/test/test-utils';

describe('TodayRevenueHeader', () => {
  it('renders as "Ingresos de hoy" — renamed from "Control de caja", which this card never was', () => {
    renderWithProviders(<TodayRevenueHeader todayRevenue={150000} yesterdayRevenue={100000} />);
    expect(screen.getByText('Ingresos de hoy')).toBeInTheDocument();
    expect(screen.queryByText('Control de caja')).not.toBeInTheDocument();
    expect(screen.getByText('$1.500')).toBeInTheDocument();
  });
});
