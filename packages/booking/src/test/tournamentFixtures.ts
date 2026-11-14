import type {
  PublicPairRegistration,
  PublicTournament,
  PublicTournamentCategory,
  PublicTournamentDetail,
  PublicTournamentGroup,
  PublicTournamentMatch,
  PublicTournamentPair,
  PublicTournamentSummary,
} from '../api/types';

export const TOURNAMENT: PublicTournament = {
  id: 't1',
  name: 'Torneo de primavera',
  sport: 'padel',
  start_date: '2099-12-05',
  end_date: '2099-12-06',
  status: 'active',
  cancel_reason: null,
  cancelled_at: null,
  created_at: '2099-11-01T12:00:00Z',
};

/** Open category in the groups format: nothing is drawn yet. */
export const CAT_OPEN: PublicTournamentCategory = {
  id: 'c-open',
  tournament_id: 't1',
  name: '8va',
  format: 'groups',
  capacity: 12,
  price: 2_000_000,
  service_fee: 200_000,
  total_price: 2_200_000,
  spots_left: 5,
  confirmed_count: 7,
  held_count: 0,
  group_size: 3,
  qualifiers_per_group: 2,
  sets_to_win: 2,
  games_per_set: 6,
  super_tiebreak_decider: true,
  match_duration_minutes: 90,
  status: 'open',
  draw_generated: false,
};

/** Drawn category in the groups format: two groups of three and a four-pair bracket. */
export const CAT_GROUPS: PublicTournamentCategory = {
  ...CAT_OPEN,
  id: 'c-groups',
  name: '6ta',
  capacity: 6,
  spots_left: 0,
  status: 'closed',
  draw_generated: true,
};

/** Drawn category in the elimination format: three pairs, so one bye. */
export const CAT_ELIM: PublicTournamentCategory = {
  ...CAT_OPEN,
  id: 'c-elim',
  name: '7ma',
  format: 'elimination',
  capacity: 4,
  spots_left: 0,
  status: 'closed',
  draw_generated: true,
};

function pair(id: string, categoryId: string, groupId: string | null, a: [string, string], b: [string, string]): PublicTournamentPair {
  return {
    id,
    category_id: categoryId,
    players: [
      { first_name: a[0], last_name: a[1] },
      { first_name: b[0], last_name: b[1] },
    ],
    seed: null,
    group_id: groupId,
    status: 'active',
  };
}

export const PAIRS: PublicTournamentPair[] = [
  pair('p1', 'c-groups', 'gA', ['Ana', 'Pérez'], ['Luis', 'Gómez']),
  pair('p2', 'c-groups', 'gA', ['Beto', 'Ruiz'], ['Caro', 'Sosa']),
  pair('p3', 'c-groups', 'gA', ['Dani', 'Vega'], ['Eli', 'Mora']),
  pair('p4', 'c-groups', 'gB', ['Fede', 'Paz'], ['Gabi', 'Luna']),
  pair('p5', 'c-groups', 'gB', ['Hugo', 'Sal'], ['Ines', 'Rey']),
  pair('p6', 'c-groups', 'gB', ['Juan', 'Cruz'], ['Kari', 'Lara']),
  pair('q1', 'c-elim', null, ['Lola', 'Nieto'], ['Mati', 'Ocampo']),
  pair('q2', 'c-elim', null, ['Nico', 'Pons'], ['Olga', 'Quiroga']),
  pair('q3', 'c-elim', null, ['Pablo', 'Rojas'], ['Rita', 'Soto']),
];

const BASE = { walkover_loser: 0, schedule: null, source1: null, source2: null, group_id: null, winner_pair_id: null, sets: [], bye: false, status: 'pending' } as const;

function match(partial: Partial<PublicTournamentMatch> & Pick<PublicTournamentMatch, 'id' | 'category_id' | 'stage' | 'round' | 'position'>): PublicTournamentMatch {
  return { pair1_id: null, pair2_id: null, ...BASE, ...partial } as PublicTournamentMatch;
}

export const MATCHES: PublicTournamentMatch[] = [
  // Group A, matchday 1: played.
  match({
    id: 'm-a1', category_id: 'c-groups', stage: 'group', group_id: 'gA', round: 1, position: 1,
    pair1_id: 'p1', pair2_id: 'p2', status: 'completed', winner_pair_id: 'p1',
    sets: [{ a: 6, b: 4 }, { a: 6, b: 3 }],
    schedule: { court_id: 'k1', court_name: 'Cancha 1', date: '2099-12-05', start_time: '10:00', end_time: '11:30', duration_minutes: 90 },
  }),
  // Group A, matchday 2: scheduled, not played.
  match({
    id: 'm-a2', category_id: 'c-groups', stage: 'group', group_id: 'gA', round: 2, position: 1,
    pair1_id: 'p1', pair2_id: 'p3',
    schedule: { court_id: 'k2', court_name: 'Cancha 2', date: '2099-12-05', start_time: '12:00', end_time: '13:30', duration_minutes: 90 },
  }),
  // Group B: one match with a super tiebreak.
  match({
    id: 'm-b1', category_id: 'c-groups', stage: 'group', group_id: 'gB', round: 1, position: 1,
    pair1_id: 'p4', pair2_id: 'p5', status: 'completed', winner_pair_id: 'p5',
    sets: [{ a: 6, b: 4 }, { a: 3, b: 6 }, { a: 8, b: 10, super: true }],
  }),
  // Bracket of the groups category: semifinals wait for the groups, then the final.
  match({
    id: 'b-s1', category_id: 'c-groups', stage: 'bracket', round: 1, position: 1,
    source1: { group_id: 'gA', rank: 1 }, source2: { group_id: 'gB', rank: 2 },
  }),
  match({
    id: 'b-s2', category_id: 'c-groups', stage: 'bracket', round: 1, position: 2,
    source1: { group_id: 'gB', rank: 1 }, source2: { group_id: 'gA', rank: 2 },
  }),
  match({ id: 'b-f', category_id: 'c-groups', stage: 'bracket', round: 2, position: 1 }),
  // Bracket of the elimination category: q1 has a bye, q2 beat q3 by walkover.
  match({
    id: 'e-s1', category_id: 'c-elim', stage: 'bracket', round: 1, position: 1,
    pair1_id: 'q1', bye: true, status: 'completed', winner_pair_id: 'q1',
  }),
  match({
    id: 'e-s2', category_id: 'c-elim', stage: 'bracket', round: 1, position: 2,
    pair1_id: 'q2', pair2_id: 'q3', status: 'completed', winner_pair_id: 'q2', walkover_loser: 2,
  }),
  match({ id: 'e-f', category_id: 'c-elim', stage: 'bracket', round: 2, position: 1, pair1_id: 'q1', pair2_id: 'q2' }),
];

export const DETAIL: PublicTournamentDetail = {
  tournament: TOURNAMENT,
  categories: [CAT_OPEN, CAT_GROUPS, CAT_ELIM],
  pairs: PAIRS,
  groups: [
    {
      id: 'gA', category_id: 'c-groups', name: 'A', position: 1,
      standings: [
        { pair_id: 'p1', rank: 1, played: 1, won: 1, lost: 0, sets_for: 2, sets_against: 0, games_for: 12, games_against: 7 },
        { pair_id: 'p3', rank: 2, played: 0, won: 0, lost: 0, sets_for: 0, sets_against: 0, games_for: 0, games_against: 0 },
        { pair_id: 'p2', rank: 3, played: 1, won: 0, lost: 1, sets_for: 0, sets_against: 2, games_for: 7, games_against: 12 },
      ],
    },
    {
      id: 'gB', category_id: 'c-groups', name: 'B', position: 2,
      standings: [
        { pair_id: 'p5', rank: 1, played: 1, won: 1, lost: 0, sets_for: 2, sets_against: 1, games_for: 10, games_against: 12 },
        { pair_id: 'p6', rank: 2, played: 0, won: 0, lost: 0, sets_for: 0, sets_against: 0, games_for: 0, games_against: 0 },
        { pair_id: 'p4', rank: 3, played: 1, won: 0, lost: 1, sets_for: 1, sets_against: 2, games_for: 12, games_against: 10 },
      ],
    },
  ],
  matches: MATCHES,
  complex_name: 'Club Norte',
  cancellation_hours: 24,
};

export const SUMMARY: PublicTournamentSummary = { ...TOURNAMENT, categories: [CAT_OPEN, CAT_GROUPS] };

export function pairRegistration(overrides: Partial<PublicPairRegistration> = {}): PublicPairRegistration {
  return {
    status: 'confirmed',
    pair_status: 'active',
    held_until: '2099-12-01T10:00:00-03:00',
    payer: { first_name: 'Ana', last_name: 'Pérez' },
    partner: { first_name: 'Luis', last_name: 'Gómez' },
    price: 2_000_000,
    service_fee: 200_000,
    tournament: TOURNAMENT,
    category: CAT_OPEN,
    complex_name: 'Club Norte',
    complex_address: 'Av. Siempre Viva 123',
    cancellation: { can_cancel: true, within_window: true, deadline: '2099-12-04T19:00:00-03:00', walkover: false },
    ...overrides,
  };
}

function required<T>(value: T | undefined, what: string): T {
  if (value === undefined) throw new Error(`fixture not found: ${what}`);
  return value;
}

export function matchById(id: string): PublicTournamentMatch {
  return required(MATCHES.find((m) => m.id === id), id);
}

export function groupById(id: string): PublicTournamentGroup {
  return required(DETAIL.groups.find((g) => g.id === id), id);
}
