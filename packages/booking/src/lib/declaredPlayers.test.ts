import { describe, expect, it } from 'vitest';
import type { Problem } from './ApiError';
import {
  declarationRefusalCopy,
  openPlayIssues,
  openPlayRequirement,
  pairIssues,
  pairRequirement,
  toDeclared,
} from './declaredPlayers';
import { BOOKING_MESSAGES } from '../messages';

const t = BOOKING_MESSAGES.declaration;

const legacy = { modality: null, level: null, suma: null };
const men8 = { modality: 'men' as const, level: 8, suma: null };
const men7 = { modality: 'men' as const, level: 7, suma: null };
const mixedSuma15 = { modality: 'mixed' as const, level: null, suma: 15 };
const womenFourth = { modality: 'women' as const, level: 4, suma: null };

function problem(errors: { field: string; message: string }[], status = 422): Pick<Problem, 'status' | 'errors'> {
  return { status, errors };
}

describe('pairIssues', () => {
  it('takes any pair for a category that predates the traits', () => {
    expect(pairIssues(legacy, { gender: 'men', level: 1 }, { gender: 'women', level: 9 })).toEqual([]);
  });

  it('needs two men for a men category', () => {
    expect(pairIssues(men8, { gender: 'men', level: 8 }, { gender: 'women', level: 8 })).toEqual([
      { field: 'partner.gender', message: t.pairMenOnly },
    ]);
  });

  it('needs two women for a women category', () => {
    const issues = pairIssues(womenFourth, { gender: 'women', level: 4 }, { gender: 'men', level: 4 });
    expect(issues).toEqual([{ field: 'partner.gender', message: t.pairWomenOnly }]);
  });

  it('needs one man and one woman for a mixed category', () => {
    const mixed = { modality: 'mixed' as const, level: 6, suma: null };
    expect(pairIssues(mixed, { gender: 'men', level: 6 }, { gender: 'men', level: 6 })).toEqual([
      { field: 'partner.gender', message: t.pairMixed },
    ]);
    expect(pairIssues(mixed, { gender: 'men', level: 6 }, { gender: 'women', level: 6 })).toEqual([]);
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
    expect(openPlayIssues({ modality: null, level_min: null, level_max: null }, { gender: 'women', level: 9 })).toEqual([]);
  });

  it('refuses a gender the modality does not take', () => {
    const men = { modality: 'men' as const, level_min: null, level_max: null };
    expect(openPlayIssues(men, { gender: 'women', level: 3 })).toEqual([
      { field: 'gender', message: 'Esta cancha abierta es solo de Caballeros.' },
    ]);
    const women = { modality: 'women' as const, level_min: null, level_max: null };
    expect(openPlayIssues(women, { gender: 'men', level: 3 })).toEqual([
      { field: 'gender', message: 'Esta cancha abierta es solo de Damas.' },
    ]);
  });

  it('takes both genders in a mixed event', () => {
    const mixed = { modality: 'mixed' as const, level_min: null, level_max: null };
    expect(openPlayIssues(mixed, { gender: 'women', level: 3 })).toEqual([]);
    expect(openPlayIssues(mixed, { gender: 'men', level: 3 })).toEqual([]);
  });

  it('keeps the level inside the range, both ends included', () => {
    const range = { modality: 'men' as const, level_min: 6, level_max: 8 };
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
    const solo = { modality: null, level_min: 4, level_max: 4 };
    expect(openPlayIssues(solo, { level: 5 })).toEqual([{ field: 'level', message: 'Esta cancha abierta es solo de 4ta.' }]);
    expect(openPlayIssues(solo, { level: 4 })).toEqual([]);
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

  it('states an open play by its modality and range', () => {
    expect(openPlayRequirement({ modality: 'men', level_min: 6, level_max: 8 })).toBe('Caballeros · de 6ta a 8va');
    expect(openPlayRequirement({ modality: 'women', level_min: 4, level_max: 4 })).toBe('Damas · solo 4ta');
    expect(openPlayRequirement({ modality: null, level_min: null, level_max: null })).toBeNull();
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
    expect(copy).toBe(t.pairMenOnly);
  });

  it('maps an open-play range and gender the server refused', () => {
    const event = { modality: 'women' as const, level_min: 6, level_max: 8 };
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
