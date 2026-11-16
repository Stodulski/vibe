import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ReviewStep } from './ReviewStep';
import type { TournamentFormValues } from '../../schemas/tournamentForm.schema';

const values = {
  name: 'Torneo de diciembre',
  sport: 'padel',
  start_date: '2026-12-12',
  end_date: '2026-12-13',
  categories: [],
} as unknown as TournamentFormValues;

describe('ReviewStep', () => {
  it('capitalizes the tournament days on a block-level box so the first letter takes effect', () => {
    render(<ReviewStep values={values} error={undefined} pending={false} onBack={vi.fn()} />);

    const days = screen.getByText(/sábado 12 al domingo 13 de diciembre/);
    expect(days).toHaveClass('inline-block', 'first-letter:uppercase');
  });
});
