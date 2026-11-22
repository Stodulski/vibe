import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { es } from 'date-fns/locale';
import { Calendar } from './calendar';

// The owner app's calendars pass date-fns' `es`, which names months and days
// but carries none of DayPicker's own labels.
describe('Calendar accessible names', () => {
  it('announces today in Spanish', () => {
    render(<Calendar mode="single" locale={es} today={new Date(2026, 10, 21)} month={new Date(2026, 10, 1)} />);

    expect(screen.getByRole('button', { name: /^Hoy, sábado, 21 de noviembre de 2026/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Today/ })).not.toBeInTheDocument();
  });

  it('names the month navigation in Spanish', () => {
    render(<Calendar mode="single" locale={es} month={new Date(2026, 10, 1)} />);

    expect(screen.getByRole('button', { name: 'Ir al mes siguiente' })).toBeInTheDocument();
  });
});
