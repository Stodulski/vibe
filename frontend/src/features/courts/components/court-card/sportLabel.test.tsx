import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { CardHeader } from './CardHeader';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CourtWithPrices, Sport } from '@/shared/types/api.types';

// Matches anything in the emoji planes plus the Miscellaneous Symbols and
// Dingbats blocks, which is where 🎾 ⚽ 🏀 and their neighbours live.
const EMOJI = /[\u{1F300}-\u{1FAFF}\u{2600}-\u{27BF}]/u;

const SPORTS: Sport[] = ['padel', 'tennis', 'soccer', 'basketball', 'volleyball', 'hockey', 'pickleball'];

function courtOf(sport: Sport): CourtWithPrices {
  return {
    id: 'ct1',
    complex_id: 'c1',
    name: 'Cancha 1',
    sport,
    court_type: 'outdoor',
    is_active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices: [],
  };
}

function renderHeader(sport: Sport) {
  return render(
    <CardHeader
      court={courtOf(sport)}
      onToggleActive={() => {
        /* not under test */
      }}
      isTogglePending={false}
    />,
  );
}

// A court card says its sport in words, and carries no emoji.
//
// There used to be a badge holding one — 🎾 for both padel and tennis, ⚽, 🏀.
// An emoji renders in whatever colour and drawing the viewer's platform ships,
// so it could never take the brand colour and was a different mark on Apple,
// Android and Windows. None of that shows up in a snapshot taken on one
// machine, which is why this asserts on the absence of the character class.
describe('the sport on a court card', () => {
  it.each(SPORTS)('names %s in words, with no emoji', (sport) => {
    const { container } = renderHeader(sport);

    expect(container.textContent).toContain(ES_AR.courts.sportTypes[sport]);
    expect(container.textContent).not.toMatch(EMOJI);
  });

  // What the card SAYS about a court is worth a test; what colour it says it
  // in is not. There was an assertion here that the sport and the court type
  // carried `text-primary-400`, and it earned its keep twice over as a
  // cautionary tale and never once as a test: it passed when the colour was
  // there, passed when the colour moved to a child element, and passed when
  // the colour was removed entirely. A restyle would break it; a real
  // regression would not.
  it('names the court type alongside the sport', () => {
    const { container } = renderHeader('padel');

    expect(container.textContent).toContain(ES_AR.courts.sportTypes.padel);
    expect(container.textContent).toContain(ES_AR.courts.courtTypes.outdoor);
  });

  // The state is the switch's to tell. The word beside it repeated the knob's
  // own position on every card of a full grid.
  it('does not repeat the active state as a word', () => {
    const { container } = renderHeader('padel');

    expect(container.textContent).not.toContain(ES_AR.courts.active);
  });
});
