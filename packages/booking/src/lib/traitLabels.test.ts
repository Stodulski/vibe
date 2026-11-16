import { describe, expect, it } from 'vitest';
import { CAT_OPEN } from '../test/tournamentFixtures';
import { categoryTitle, levelLabel, openPlayTitle, tournamentLabel } from './traitLabels';

describe('levelLabel', () => {
  it('names the nine levels from 1ra to 9na', () => {
    const names = [1, 2, 3, 4, 5, 6, 7, 8, 9].map(levelLabel);
    expect(names).toEqual(['1ra', '2da', '3ra', '4ta', '5ta', '6ta', '7ma', '8va', '9na']);
  });
});

describe('categoryTitle', () => {
  it('composes the modality and the level', () => {
    expect(categoryTitle({ ...CAT_OPEN, name: 'stored', modality: 'men', level: 8, suma: null })).toBe('Caballeros 8va');
    expect(categoryTitle({ ...CAT_OPEN, name: 'stored', modality: 'women', level: 5, suma: null })).toBe('Damas 5ta');
    expect(categoryTitle({ ...CAT_OPEN, name: 'stored', modality: 'mixed', level: 6, suma: null })).toBe('Mixto 6ta');
  });

  it('composes a Suma with its number', () => {
    expect(categoryTitle({ ...CAT_OPEN, name: 'stored', modality: 'mixed', level: null, suma: 15 })).toBe('Mixto Suma 15');
  });

  it('keeps the stored name for a category created before the traits', () => {
    expect(categoryTitle(CAT_OPEN)).toBe('8va');
  });

  it('keeps the stored name when the modality is there but neither a level nor a Suma', () => {
    expect(categoryTitle({ ...CAT_OPEN, name: 'stored', modality: 'men', level: null, suma: null })).toBe('stored');
  });
});

describe('openPlayTitle', () => {
  it('names the sport when the event has no modality or range', () => {
    expect(openPlayTitle({ sport: 'padel', modality: null, level_min: null, level_max: null })).toBe(
      'Cancha abierta de pádel',
    );
  });

  it('adds the modality and the level range', () => {
    expect(openPlayTitle({ sport: 'padel', modality: 'men', level_min: 6, level_max: 8 })).toBe(
      'Cancha abierta de pádel · Caballeros · 6ta a 8va',
    );
  });

  it('writes a range of one level as solo', () => {
    expect(openPlayTitle({ sport: 'padel', modality: 'women', level_min: 4, level_max: 4 })).toBe(
      'Cancha abierta de pádel · Damas · solo 4ta',
    );
  });

  it('keeps a mixed event named by its modality', () => {
    expect(openPlayTitle({ sport: 'padel', modality: 'mixed', level_min: null, level_max: null })).toBe(
      'Cancha abierta de pádel · Mixto',
    );
  });
});

describe('tournamentLabel', () => {
  it('names the sport in lower case', () => {
    expect(tournamentLabel('padel')).toBe('Torneo de pádel');
    expect(tournamentLabel('soccer')).toBe('Torneo de fútbol');
  });
});
