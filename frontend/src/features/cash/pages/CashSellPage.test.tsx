import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import CashSellPage from './CashSellPage';

describe('CashSellPage', () => {
  it('renders the Vender placeholder with the section tabs', () => {
    render(
      <MemoryRouter initialEntries={['/cash/sell']}>
        <CashSellPage />
      </MemoryRouter>,
    );

    expect(
      screen.getByText('Esta pantalla todavía no está lista. Muy pronto vas a poder vender desde acá.'),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Vender' })).toHaveClass('border-primary-500');
    expect(screen.getByRole('link', { name: 'Turno' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Productos' })).toBeInTheDocument();
  });
});
