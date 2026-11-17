import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeMatch, makePair } from '../testing/fixtures';
import { BracketSection } from './BracketSection';

const pairs = new Map(['p1', 'p2', 'p3', 'p4'].map((id) => [id, makePair({ id })]));

function connectors(container: HTMLElement, kind: 'out' | 'in') {
  return container.querySelectorAll(`[data-bracket-connector="${kind}"]`);
}

describe('BracketSection connectors', () => {
  it('joins each match to the match of the next round it feeds', () => {
    const matches = [
      makeMatch({ id: 'a', round: 1, position: 1, pair1Id: 'p1', pair2Id: 'p2' }),
      makeMatch({ id: 'b', round: 1, position: 2, pair1Id: 'p3', pair2Id: 'p4' }),
      makeMatch({ id: 'c', round: 2, position: 1 }),
    ];
    const { container } = render(<BracketSection matches={matches} pairs={pairs} groupNames={new Map()} />);

    expect(screen.getByRole('region', { name: ES_AR.tournaments.bracket.label })).toBeInTheDocument();
    expect(connectors(container, 'out')).toHaveLength(2);
    expect(connectors(container, 'in')).toHaveLength(1);
  });

  it('draws none for a single match', () => {
    const matches = [makeMatch({ id: 'c', round: 1, position: 1, pair1Id: 'p1', pair2Id: 'p2' })];
    const { container } = render(<BracketSection matches={matches} pairs={pairs} groupNames={new Map()} />);

    expect(connectors(container, 'out')).toHaveLength(0);
    expect(connectors(container, 'in')).toHaveLength(0);
  });

  it('hides the connectors from assistive technology', () => {
    const matches = [
      makeMatch({ id: 'a', round: 1, position: 1, pair1Id: 'p1', pair2Id: 'p2' }),
      makeMatch({ id: 'b', round: 1, position: 2, pair1Id: 'p3', pair2Id: 'p4' }),
      makeMatch({ id: 'c', round: 2, position: 1 }),
    ];
    const { container } = render(<BracketSection matches={matches} pairs={pairs} groupNames={new Map()} />);

    for (const node of container.querySelectorAll('[data-bracket-connector]')) {
      expect(node).toHaveAttribute('aria-hidden', 'true');
    }
  });
});

describe('BracketSection match line', () => {
  it('writes the schedule on one truncated line with the day in figures', () => {
    const schedule = {
      court_id: 'ct1',
      court_name: 'Pádel 3',
      date: '2026-11-22',
      start_time: '14:00',
      end_time: '15:30',
      duration_minutes: 90,
    };
    const matches = [makeMatch({ id: 'a', round: 1, position: 1, pair1Id: 'p1', pair2Id: 'p2', schedule })];
    render(<BracketSection matches={matches} pairs={pairs} groupNames={new Map()} />);

    expect(screen.getByText(/Dom 22\/11/)).toHaveClass('truncate');
    expect(screen.queryByText(/domingo/)).not.toBeInTheDocument();
  });

  it('keeps each pair name on one truncated line', () => {
    const matches = [makeMatch({ id: 'a', round: 1, position: 1, pair1Id: 'p1', pair2Id: 'p2' })];
    render(<BracketSection matches={matches} pairs={pairs} groupNames={new Map()} />);

    const slot = screen.getAllByText(/ \/ /)[0];
    if (!slot) throw new Error('the match shows no pair name');
    expect(slot).toHaveClass('truncate');
  });
});

describe('BracketSection geometry', () => {
  const eight = new Map(
    Array.from({ length: 8 }, (_, i) => [`p${String(i + 1)}`, makePair({ id: `p${String(i + 1)}` })]),
  );
  const matches = [
    ...[1, 2, 3, 4].map((n) =>
      makeMatch({
        id: `q${String(n)}`,
        round: 1,
        position: n,
        pair1Id: `p${String(n * 2 - 1)}`,
        pair2Id: `p${String(n * 2)}`,
      }),
    ),
    makeMatch({ id: 's1', round: 2, position: 1 }),
    makeMatch({ id: 's2', round: 2, position: 2 }),
    makeMatch({ id: 'f', round: 3, position: 1 }),
  ];

  it('gives every round column the same rows and each match the share of its feeders', () => {
    render(<BracketSection matches={matches} pairs={eight} groupNames={new Map()} />);
    const lists = screen.getAllByRole('list');
    expect(lists).toHaveLength(3);
    for (const list of lists) expect(list).toHaveStyle({ gridRow: '2 / span 4' });
    expect(lists.map((list) => (list.firstElementChild as HTMLElement).style.gridRow)).toEqual([
      '1 / span 1',
      '1 / span 2',
      '1 / span 4',
    ]);
    expect((lists[1]?.children[1] as HTMLElement).style.gridRow).toBe('3 / span 2');
  });

  it('joins each feeder pair with a stub, a vertical line and a stub into the next match', () => {
    const { container } = render(<BracketSection matches={matches} pairs={eight} groupNames={new Map()} />);
    const out = [...container.querySelectorAll('[data-bracket-connector="out"]')];
    expect(out).toHaveLength(6);
    expect(out.filter((node) => node.classList.contains('border-t'))).toHaveLength(3);
    expect(out.filter((node) => node.classList.contains('border-b'))).toHaveLength(3);
    for (const node of out) expect(node).toHaveClass('border-r');
    expect(connectors(container, 'in')).toHaveLength(3);
  });
});
