import { describe, expect, it } from 'vitest';
import type { Problem } from './ApiError';
import {
  declarationRefusalCopy,
  openPlayDeclaresLevel,
  openPlayIssues,
  openPlayRequirement,
  pairDeclaresLevel,
  pairIssues,
  pairRequirement,
  toDeclared,
} from './declaredPlayers';
import { BOOKING_MESSAGES } from '../messages';

const t = BOOKING_MESSAGES.declaration;

const legacy = { sport: 'padel', modality: null, level: null, suma: null };
const men8 = { sport: 'padel', modality: 'men' as const, level: 8, suma: null };
const men7 = { sport: 'padel', modality: 'men' as const, level: 7, suma: null };
const mixedSuma15 = { sport: 'padel', modality: 'mixed' as const, level: null, suma: 15 };
const womenFourth = { sport: 'padel', modality: 'women' as const, level: 4, suma: null };

function problem(errors: { field: string; message: string }[], status = 422): Pick<Problem, 'status' | 'errors'> {
  return { status, errors };
}

describe('pairIssues', () => {
  it('takes any pair for a category that predates the traits', () => {
    expect(pairIssues(legacy, { gender: 'men', level: 1 }, { gender: 'women', level: 9 })).toEqual([]);
  });

  it('needs two men for a men category', () => {
    expect(pairIssues(men8, { gender: 'men', level: 8 }, { gender: 'women', level: 8 })).toEqual([
      { field: 'partner.gender', message: t.pairOnly('Caballeros', 'caballeros') },
    ]);
  });

  it('needs two women for a women category', () => {
    const issues = pairIssues(womenFourth, { gender: 'women', level: 4 }, { gender: 'men', level: 4 });
    expect(issues).toEqual([{ field: 'partner.gender', message: t.pairOnly('Damas', 'damas') }]);
  });

  it('needs one man and one woman for a mixed category', () => {
    const mixed = { sport: 'padel', modality: 'mixed' as const, level: 6, suma: null };
    expect(pairIssues(mixed, { gender: 'men', level: 6 }, { gender: 'men', level: 6 })).toEqual([
      { field: 'partner.gender', message: t.pairMixed('un caballero y una dama') },
    ]);
    expect(pairIssues(mixed, { gender: 'men', level: 6 }, { gender: 'women', level: 6 })).toEqual([]);
  });

  it('words a fútbol category with its own modality and players', () => {
    const soccerMen = { sport: 'soccer', modality: 'men' as const, level: null, suma: null };
    expect(pairIssues(soccerMen, { gender: 'men' }, { gender: 'women' })).toEqual([
      { field: 'partner.gender', message: 'Esta categoría es de Masculino: los dos jugadores tienen que ser masculinos.' },
    ]);
  });

  it('refuses a player stronger than a level category, the way a 6ta cannot enter an 8va', () => {
    expect(pairIssues(men8, { gender: 'men', level: 6 }, { gender: 'men', level: 8 })).toEqual([
      { field: 'level', message: 'Esta categoría es 8va: no pueden anotarse jugadores de una categoría más alta.' },
    ]);
  });

  it('takes an equal or weaker player in a level category', () => {
    expect(pairIssues(men7, { gender: 'men', level: 7 }, { gender: 'men', level: 9 })).toEqual([]);
  });

  it('refuses the partner too when the partner is stronger', () => {
    const issues = pairIssues(men7, { gender: 'men', level: 8 }, { gender: 'men', level: 5 });
    expect(issues).toEqual([{ field: 'partner.level', message: expect.stringContaining('7ma') as string }]);
  });

  it('needs the two levels to add up to a Suma', () => {
    expect(pairIssues(mixedSuma15, { gender: 'men', level: 7 }, { gender: 'women', level: 7 })).toEqual([
      { field: 'partner.level', message: t.sumaTooLow(15) },
    ]);
    expect(pairIssues(mixedSuma15, { gender: 'men', level: 7 }, { gender: 'women', level: 8 })).toEqual([]);
  });

  it('leaves a level it has not got yet to the form', () => {
    expect(pairIssues(men8, { gender: 'men' }, { gender: 'men', level: 8 })).toEqual([]);
  });
});

describe('openPlayIssues', () => {
  it('takes anyone for an event without a modality', () => {
    expect(
      openPlayIssues({ sport: 'padel', modality: null, level_min: null, level_max: null }, { gender: 'women', level: 9 }),
    ).toEqual([]);
  });

  it('refuses a gender the modality does not take', () => {
    const men = { sport: 'padel', modality: 'men' as const, level_min: null, level_max: null };
    expect(openPlayIssues(men, { gender: 'women', level: 3 })).toEqual([
      { field: 'gender', message: 'Esta cancha abierta es solo de Caballeros.' },
    ]);
    const women = { sport: 'padel', modality: 'women' as const, level_min: null, level_max: null };
    expect(openPlayIssues(women, { gender: 'men', level: 3 })).toEqual([
      { field: 'gender', message: 'Esta cancha abierta es solo de Damas.' },
    ]);
  });

  it('names the gender of a fútbol event with its own words', () => {
    const soccerWomen = { sport: 'soccer', modality: 'women' as const, level_min: null, level_max: null };
    expect(openPlayIssues(soccerWomen, { gender: 'men' })).toEqual([
      { field: 'gender', message: 'Esta cancha abierta es solo de Femenino.' },
    ]);
  });

  it('takes both genders in a mixed event', () => {
    const mixed = { sport: 'padel', modality: 'mixed' as const, level_min: null, level_max: null };
    expect(openPlayIssues(mixed, { gender: 'women', level: 3 })).toEqual([]);
    expect(openPlayIssues(mixed, { gender: 'men', level: 3 })).toEqual([]);
  });

  it('keeps the level inside the range, both ends included', () => {
    const range = { sport: 'padel', modality: 'men' as const, level_min: 6, level_max: 8 };
    expect(openPlayIssues(range, { gender: 'men', level: 6 })).toEqual([]);
    expect(openPlayIssues(range, { gender: 'men', level: 8 })).toEqual([]);
    expect(openPlayIssues(range, { gender: 'men', level: 5 })).toEqual([
      { field: 'level', message: 'Esta cancha abierta es de 6ta a 8va.' },
    ]);
    expect(openPlayIssues(range, { gender: 'men', level: 9 })).toEqual([
      { field: 'level', message: 'Esta cancha abierta es de 6ta a 8va.' },
    ]);
  });

  it('names a single level as the only one', () => {
    const solo = { sport: 'padel', modality: null, level_min: 4, level_max: 4 };
    expect(openPlayIssues(solo, { level: 5 })).toEqual([{ field: 'level', message: 'Esta cancha abierta es solo de 4ta.' }]);
    expect(openPlayIssues(solo, { level: 4 })).toEqual([]);
  });
});

describe('level declarations', () => {
  it('asks a level only of a category with a level or a Suma', () => {
    expect(pairDeclaresLevel(men7)).toBe(true);
    expect(pairDeclaresLevel(mixedSuma15)).toBe(true);
    expect(pairDeclaresLevel({ sport: 'soccer', modality: 'men', level: null, suma: null })).toBe(false);
    expect(pairDeclaresLevel(legacy)).toBe(false);
  });

  it('asks a level only of an event with a range', () => {
    expect(openPlayDeclaresLevel({ sport: 'padel', modality: 'men', level_min: 6, level_max: 8 })).toBe(true);
    expect(openPlayDeclaresLevel({ sport: 'soccer', modality: 'men', level_min: null, level_max: null })).toBe(false);
  });
});

describe('requirements', () => {
  it('states a level category as its level or weaker', () => {
    expect(pairRequirement(men7)).toBe('Caballeros · 7ma o menor');
  });

  it('states a Suma as the sum it needs', () => {
    expect(pairRequirement(mixedSuma15)).toBe(
      'Mixto · Suma 15: la suma de las dos categorías tiene que ser 15 o más.',
    );
  });

  it('states nothing for a category that predates the traits', () => {
    expect(pairRequirement(legacy)).toBeNull();
  });

  it('states a fútbol category by its modality alone', () => {
    expect(pairRequirement({ sport: 'soccer', modality: 'men', level: null, suma: null })).toBe('Masculino');
  });

  it('states an open play by its modality and range', () => {
    expect(openPlayRequirement({ sport: 'padel', modality: 'men', level_min: 6, level_max: 8 })).toBe(
      'Caballeros · de 6ta a 8va',
    );
    expect(openPlayRequirement({ sport: 'padel', modality: 'women', level_min: 4, level_max: 4 })).toBe('Damas · solo 4ta');
    expect(openPlayRequirement({ sport: 'padel', modality: null, level_min: null, level_max: null })).toBeNull();
  });

  it('states a fútbol open play by its modality alone', () => {
    expect(openPlayRequirement({ sport: 'soccer', modality: 'women', level_min: null, level_max: null })).toBe('Femenino');
  });
});

describe('declarationRefusalCopy', () => {
  it('maps a level the server refused to the category copy', () => {
    const copy = declarationRefusalCopy(problem([{ field: 'partner.level', message: 'level_too_strong' }]), {
      kind: 'pair',
      category: men8,
    });
    expect(copy).toBe('Esta categoría es 8va: no pueden anotarse jugadores de una categoría más alta.');
  });

  it('maps a Suma the server refused', () => {
    const copy = declarationRefusalCopy(problem([{ field: 'levels', message: 'suma_too_low' }]), {
      kind: 'pair',
      category: mixedSuma15,
    });
    expect(copy).toBe(t.sumaTooLow(15));
  });

  it('maps a modality the server refused to the copy of that modality', () => {
    const copy = declarationRefusalCopy(problem([{ field: 'modality', message: 'modality_mismatch' }]), {
      kind: 'pair',
      category: men8,
    });
    expect(copy).toBe(t.pairOnly('Caballeros', 'caballeros'));
  });

  it('maps an open-play range and gender the server refused', () => {
    const event = { sport: 'padel', modality: 'women' as const, level_min: 6, level_max: 8 };
    expect(declarationRefusalCopy(problem([{ field: 'level', message: 'level_out_of_range' }]), { kind: 'event', event })).toBe(
      'Esta cancha abierta es de 6ta a 8va.',
    );
    expect(declarationRefusalCopy(problem([{ field: 'gender', message: 'modality_mismatch' }]), { kind: 'event', event })).toBe(
      'Esta cancha abierta es solo de Damas.',
    );
  });

  it('says nothing for a refusal that names no declaration', () => {
    expect(declarationRefusalCopy(problem([{ field: 'turnstile_token', message: 'invalid' }]), { kind: 'pair', category: men8 })).toBeUndefined();
    expect(declarationRefusalCopy(undefined, { kind: 'pair', category: men8 })).toBeUndefined();
  });
});

describe('toDeclared', () => {
  it('reads the form strings as a declaration, and leaves what is not chosen out', () => {
    expect(toDeclared('women', '5')).toEqual({ gender: 'women', level: 5 });
    expect(toDeclared('', '')).toEqual({ gender: undefined, level: undefined });
  });
});
