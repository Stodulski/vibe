import type {
  TournamentCategory,
  TournamentDetailResponse,
  TournamentGroup,
  TournamentMatch,
  TournamentOwnerPair,
  TournamentSet,
} from '@/shared/types/api.types';

/** Test builders for the owner tournament payloads. Not shipped: only tests import them. */
export function makePair(opts: {
  id: string;
  players?: [string, string];
  registration?: 'held' | 'confirmed' | 'cancelled' | 'expired';
  payment?: string | null;
  seed?: number | null;
  groupId?: string | null;
  categoryId?: string;
  status?: 'active' | 'withdrawn';
}): TournamentOwnerPair {
  const [first = 'Ana Pérez', second = 'Bea Gómez'] = opts.players ?? [];
  const player = (full: string) => {
    const [given = '', ...rest] = full.split(' ');
    return { first_name: given, last_name: rest.join(' ') };
  };
  const owner = (full: string) => ({ ...player(full), phone: '1155550000', email: null });
  const registration = opts.registration ?? 'confirmed';
  return {
    id: opts.id,
    category_id: opts.categoryId ?? 'cat1',
    players: [player(first), player(second)],
    seed: opts.seed ?? null,
    group_id: opts.groupId ?? null,
    status: opts.status ?? 'active',
    payer: owner(first),
    partner: owner(second),
    registration: {
      status: registration,
      price: 2000000,
      service_fee: 100000,
      held_until: '2026-12-01T12:00:00Z',
      confirmed_at: registration === 'confirmed' ? '2026-12-01T11:00:00Z' : null,
      cancelled_by: null,
      signed_up_at: '2026-12-01T10:00:00Z',
    },
    payment:
      opts.payment === undefined && registration !== 'confirmed'
        ? null
        : { status: opts.payment ?? 'approved', amount: 2100000, service_fee: 100000, refund_amount: 0 },
  };
}

export function makeMatch(opts: {
  id: string;
  categoryId?: string;
  stage?: 'group' | 'bracket';
  groupId?: string | null;
  round?: number;
  position?: number;
  pair1Id?: string | null;
  pair2Id?: string | null;
  source1?: TournamentMatch['source1'];
  source2?: TournamentMatch['source2'];
  bye?: boolean;
  status?: 'pending' | 'completed';
  winnerPairId?: string | null;
  sets?: TournamentSet[];
  walkoverLoser?: 0 | 1 | 2;
  schedule?: TournamentMatch['schedule'];
}): TournamentMatch {
  return {
    id: opts.id,
    category_id: opts.categoryId ?? 'cat1',
    stage: opts.stage ?? 'bracket',
    group_id: opts.groupId ?? null,
    round: opts.round ?? 1,
    position: opts.position ?? 1,
    pair1_id: opts.pair1Id ?? null,
    pair2_id: opts.pair2Id ?? null,
    source1: opts.source1 ?? null,
    source2: opts.source2 ?? null,
    bye: opts.bye ?? false,
    status: opts.status ?? 'pending',
    winner_pair_id: opts.winnerPairId ?? null,
    sets: opts.sets ?? [],
    walkover_loser: opts.walkoverLoser ?? 0,
    schedule: opts.schedule ?? null,
  };
}

export function makeCategory(overrides: Partial<TournamentCategory> = {}): TournamentCategory {
  return {
    id: 'cat1',
    tournament_id: 't1',
    name: '6ta',
    format: 'groups',
    capacity: 16,
    price: 2000000,
    spots_left: 10,
    confirmed_count: 6,
    held_count: 0,
    group_size: 3,
    qualifiers_per_group: 2,
    sets_to_win: 2,
    games_per_set: 6,
    super_tiebreak_decider: true,
    match_duration_minutes: 90,
    status: 'open',
    draw_generated: false,
    ...overrides,
  };
}

export function makeDetail(overrides: Partial<TournamentDetailResponse> = {}): TournamentDetailResponse {
  return {
    tournament: {
      id: 't1',
      name: 'Copa Vibe',
      sport: 'padel',
      start_date: '2099-01-02',
      end_date: '2099-01-04',
      status: 'active',
      cancel_reason: null,
      cancelled_at: null,
      created_at: '2026-11-14T12:00:00Z',
    },
    categories: [makeCategory()],
    pairs: [],
    groups: [] as TournamentGroup[],
    matches: [],
    ...overrides,
  };
}

/** A drawn groups category: four pairs in one group A and a two-round bracket fed by it. */
export function makeDrawnDetail(overrides: Partial<TournamentDetailResponse> = {}): TournamentDetailResponse {
  const pairs = [
    makePair({ id: 'p1', players: ['Ana Pérez', 'Bea Gómez'], seed: 1, groupId: 'g1' }),
    makePair({ id: 'p2', players: ['Carla Ruiz', 'Dana Soto'], seed: 2, groupId: 'g1' }),
    makePair({ id: 'p3', players: ['Eva Díaz', 'Flor Vega'], seed: 3, groupId: 'g1' }),
    makePair({ id: 'p4', players: ['Gina Luna', 'Hilda Paz'], seed: 4, groupId: 'g2' }),
  ];
  const standing = (pairId: string, rank: number, won: number, lost: number) => ({
    pair_id: pairId,
    rank,
    played: won + lost,
    won,
    lost,
    sets_for: won * 2,
    sets_against: lost * 2,
    games_for: won * 12,
    games_against: lost * 12,
  });
  return makeDetail({
    categories: [makeCategory({ status: 'closed', draw_generated: true, confirmed_count: 4 })],
    pairs,
    groups: [
      {
        id: 'g1',
        category_id: 'cat1',
        name: 'A',
        position: 1,
        standings: [standing('p1', 1, 1, 0), standing('p2', 2, 0, 1), standing('p3', 3, 0, 0)],
      },
      { id: 'g2', category_id: 'cat1', name: 'B', position: 2, standings: [standing('p4', 1, 0, 0)] },
    ],
    matches: [
      makeMatch({ id: 'gm1', stage: 'group', groupId: 'g1', round: 1, pair1Id: 'p1', pair2Id: 'p2' }),
      makeMatch({
        id: 'b1',
        round: 1,
        position: 1,
        source1: { group_id: 'g1', rank: 1 },
        source2: { group_id: 'g2', rank: 2 },
      }),
      makeMatch({ id: 'b2', round: 2, position: 1 }),
    ],
    ...overrides,
  });
}
