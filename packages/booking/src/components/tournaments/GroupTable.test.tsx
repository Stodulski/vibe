import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { CAT_GROUPS, MATCHES, PAIRS, groupById } from '../../test/tournamentFixtures';
import { GroupSection } from './GroupSection';

function renderGroup(id: string) {
  return render(<GroupSection group={groupById(id)} category={CAT_GROUPS} pairs={PAIRS} matches={MATCHES} />);
}

describe('GroupSection', () => {
  it('titles the group and lists the standings best first', () => {
    renderGroup('gA');
    expect(screen.getByRole('heading', { name: 'Zona A' })).toBeInTheDocument();
    const rows = within(screen.getByRole('table', { name: 'Posiciones de la zona A' })).getAllByRole('row');
    // Header, then three pairs in rank order.
    expect(rows).toHaveLength(4);
    expect(rows[1]).toHaveTextContent('Ana Pérez');
    expect(rows[2]).toHaveTextContent('Dani Vega');
    expect(rows[3]).toHaveTextContent('Beto Ruiz');
  });

  it('shows played, won, lost, sets and games', () => {
    renderGroup('gA');
    expect(
      screen
        .getAllByRole('cell')
        .slice(0, 7)
        .map((c) => c.textContent),
    ).toEqual([
      '1',
      'Ana PérezLuis Gómez',
      '1',
      '1',
      '0',
      '2-0',
      '12-7',
    ]);
  });

  it('marks the pairs that go through to the bracket', () => {
    renderGroup('gA');
    const rows = within(screen.getByRole('table')).getAllByRole('row');
    expect(rows[1]).toHaveAttribute('data-qualifies', 'true');
    expect(rows[2]).toHaveAttribute('data-qualifies', 'true');
    expect(rows[3]).toHaveAttribute('data-qualifies', 'false');
    expect(screen.getByText('Pasan a la llave las 2 mejores parejas de cada zona.')).toBeInTheDocument();
  });

  it('lists the group matches by matchday with the result and the court', () => {
    renderGroup('gA');
    expect(screen.getByRole('heading', { name: 'Fecha 1' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Fecha 2' })).toBeInTheDocument();
    expect(screen.getByText('6-4 6-3')).toBeInTheDocument();
    expect(screen.getByText(/Cancha 2/)).toBeInTheDocument();
  });

  it('writes a match tiebreak in brackets', () => {
    renderGroup('gB');
    expect(screen.getByText('6-4 3-6 [8-10]')).toBeInTheDocument();
  });
});
