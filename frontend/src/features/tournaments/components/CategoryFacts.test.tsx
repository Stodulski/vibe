import { describe, it, expect } from 'vitest';
import { renderWithProviders, screen, userEvent } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeCategory } from '../testing/fixtures';
import { CategoryFacts } from './CategoryFacts';

const t = ES_AR.tournaments;
const NO_COURT_FEE = { collected: 0, pending: 0 };

describe('CategoryFacts', () => {
  it('reads short on a phone, without the start, and in full from sm up', () => {
    const category = makeCategory({ confirmed_count: 8, capacity: 8, price: 3000000, format: 'groups' });
    renderWithProviders(<CategoryFacts category={category} courtFee={NO_COURT_FEE} />);

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
    renderWithProviders(<CategoryFacts category={category} courtFee={NO_COURT_FEE} />);

    expect(screen.getByText('4/16 · $30.000 · Elim.')).toHaveClass('sm:hidden');
    expect(screen.getByText(`4/16 parejas · $30.000 por pareja · ${t.format.elimination}`)).toHaveClass(
      'hidden',
      'sm:inline',
    );
  });

  it('keeps the rules closed until the owner opens them, in the smaller muted text', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CategoryFacts category={makeCategory()} courtFee={NO_COURT_FEE} />);

    expect(screen.queryByText(t.detail.groupsSetup(3, 2))).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: t.detail.rules }));
    expect(screen.getByText(t.detail.groupsSetup(3, 2)).parentElement).toHaveClass('text-xs', 'text-text-tertiary');
  });
});
