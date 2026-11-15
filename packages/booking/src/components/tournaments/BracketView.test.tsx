import { describe, it, expect, afterEach, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { DETAIL, MATCHES, PAIRS, matchById } from '../../test/tournamentFixtures';
import { BracketView } from './BracketView';

const groupNames = new Map(DETAIL.groups.map((g) => [g.id, g.name]));

function bracketOf(categoryId: string) {
  return render(
    <BracketView
      matches={MATCHES.filter((m) => m.category_id === categoryId)}
      pairs={PAIRS}
      groupNames={groupNames}
    />,
  );
}

describe('BracketView', () => {
  it('draws one labelled column per round', () => {
    bracketOf('c-groups');
    expect(screen.getByRole('heading', { name: 'Semifinales' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Final' })).toBeInTheDocument();
    expect(screen.getAllByRole('listitem', { name: /^Semifinales/ })).toHaveLength(2);
  });

  it('says where a slot waiting for a group comes from', () => {
    bracketOf('c-groups');
    expect(screen.getAllByText('1º de la zona A')).toHaveLength(1);
    expect(screen.getByText('2º de la zona B')).toBeInTheDocument();
    expect(screen.getAllByText('A definir').length).toBeGreaterThan(0);
  });

  it('shows a bye as Libre and a walkover as W.O. next to the winner', () => {
    bracketOf('c-elim');
    expect(screen.getByText('Libre')).toBeInTheDocument();
    expect(screen.getByText('W.O.')).toBeInTheDocument();
    const final = screen.getByRole('listitem', { name: /^Final/ });
    expect(within(final).getByText('Lola Nieto')).toBeInTheDocument();
    expect(within(final).getByText('Nico Pons')).toBeInTheDocument();
  });

  it('marks the winner of a match loaded without a score, and shows no score', () => {
    const played = { ...matchById('e-f'), status: 'completed' as const, winner_pair_id: 'q2' };
    render(<BracketView matches={[played]} pairs={PAIRS} groupNames={groupNames} />);
    const final = screen.getByRole('listitem', { name: /^Final/ });
    expect(within(final).getByText('Ganó')).toBeInTheDocument();
    expect(within(final).getByText('Nico Pons').closest('div')).toHaveClass('font-semibold');
  });

  it('scrolls horizontally inside its own container, never the page', () => {
    bracketOf('c-groups');
    const region = screen.getByRole('region', { name: 'Cuadro' });
    expect(region).toHaveClass('overflow-x-auto');
    expect(region).toHaveAttribute('tabindex', '0');
  });

  it('renders nothing when the category has no bracket', () => {
    const { container } = render(
      <BracketView matches={MATCHES.filter((m) => m.stage === 'group')} pairs={PAIRS} groupNames={groupNames} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('draws a connector from each match to the next round, and into it', () => {
    const { container } = bracketOf('c-groups');
    expect(container.querySelectorAll('[data-bracket-connector="out"]')).toHaveLength(2);
    expect(container.querySelectorAll('[data-bracket-connector="in"]')).toHaveLength(1);
  });

  it('writes a match schedule as court, day and time', () => {
    const schedule = { court_id: 'k1', court_name: 'Cancha 1', date: '2099-12-05', start_time: '10:00', end_time: '11:30', duration_minutes: 90 };
    const matches = MATCHES.filter((m) => m.category_id === 'c-groups').map((m) =>
      m.stage === 'bracket' && m.round === 1 ? { ...m, schedule } : m,
    );
    render(<BracketView matches={matches} pairs={PAIRS} groupNames={groupNames} />);
    expect(screen.getAllByText('Cancha 1 \u00b7 sábado 5 de diciembre \u00b7 10:00').length).toBeGreaterThan(0);
  });
});

describe('BracketView, fitting the card', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('lets the columns share the card width instead of a fixed 15rem each', () => {
    bracketOf('c-groups');
    const columns = screen.getAllByRole('heading', { level: 4 }).map((heading) => heading.parentElement);
    for (const column of columns) {
      expect(column).toHaveClass('flex-1', 'basis-0');
      expect(column?.className).not.toMatch(/\bw-60\b/);
    }
  });

  it('shows no edge fade while the bracket fits', () => {
    bracketOf('c-groups');
    expect(document.querySelector('[data-bracket-fade]')).toBeNull();
  });

  it('shows a fade on the cut edge when the bracket is wider than its card', () => {
    vi.spyOn(HTMLElement.prototype, 'scrollWidth', 'get').mockReturnValue(900);
    vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(500);
    bracketOf('c-groups');
    expect(document.querySelector('[data-bracket-fade="end"]')).not.toBeNull();
    expect(document.querySelector('[data-bracket-fade="start"]')).toBeNull();
  });
});
