import { describe, it, expect } from 'vitest';
import { renderWithProviders, screen } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeCategory } from '../testing/fixtures';
import { CategoryFacts } from './CategoryFacts';

const t = ES_AR.tournaments;

describe('CategoryFacts', () => {
  it('reads short on a phone, without the start, and in full from sm up', () => {
    const category = makeCategory({ confirmed_count: 8, capacity: 8, price: 3000000, format: 'groups' });
    renderWithProviders(<CategoryFacts category={category} />);

    const short = screen.getByText('8/8 · $30.000 · Zonas');
    expect(short).toHaveClass('sm:hidden');
    expect(short).not.toHaveTextContent('18:00');

    const full = screen.getByText(/^8\/8 parejas · \$30\.000 por pareja · Zonas y cuadro/);
    expect(full).toHaveClass('hidden', 'sm:inline');
    expect(full).toHaveTextContent('18:00');
  });

  it('gives elimination the short label "Elim." on a phone', () => {
    const category = makeCategory({
      confirmed_count: 4,
      capacity: 16,
      price: 3000000,
      format: 'elimination',
      starts_on: null,
      starts_at: null,
    });
    renderWithProviders(<CategoryFacts category={category} />);

    expect(screen.getByText('4/16 · $30.000 · Elim.')).toHaveClass('sm:hidden');
    expect(screen.getByText(`4/16 parejas · $30.000 por pareja · ${t.format.elimination}`)).toHaveClass(
      'hidden',
      'sm:inline',
    );
  });

  it('shows no rules disclosure: the summary line is all the category shows', () => {
    renderWithProviders(<CategoryFacts category={makeCategory()} />);
    expect(screen.queryByRole('button', { name: /reglas/ })).not.toBeInTheDocument();
  });
});
