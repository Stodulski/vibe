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
