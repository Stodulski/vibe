import { describe, expect, it } from 'vitest';
import { CAT_OPEN } from '../test/tournamentFixtures';
import {
  categoryTitle,
  declarationWords,
  modalityLabel,
  openPlayAudience,
  openPlayLabel,
  openPlayTitle,
  tournamentLabel,
} from './traitLabels';

describe('openPlayLabel', () => {
  it('names the open play of a sport in its own noun', () => {
    expect(openPlayLabel('padel')).toBe('Cancha abierta de pádel');
    expect(openPlayLabel('soccer')).toBe('Cancha abierta de fútbol');
  });
});

describe('openPlayAudience', () => {
  it('names the modality and the level range of pádel', () => {
    expect(openPlayAudience({ sport: 'padel', modality: 'women', level_min: 6, level_max: 8 })).toBe('Damas · 6ta a 8va');
  });

  it('names a single level as solo', () => {
    expect(openPlayAudience({ sport: 'padel', modality: 'men', level_min: 4, level_max: 4 })).toBe(
      'Caballeros · solo 4ta',
    );
  });

  it('names a sport without levels by its modality alone', () => {
    expect(openPlayAudience({ sport: 'soccer', modality: 'women' })).toBe('Femenino');
  });

  it('is open to everyone when the event has no modality', () => {
    expect(openPlayAudience({ sport: 'padel' })).toBe('Abierta a todos');
    expect(openPlayAudience({ sport: 'tennis', modality: null })).toBe('Abierta a todos');
  });
});

describe('modalityLabel', () => {
  it('names the modality of pádel, tenis and pickleball as Caballeros and Damas', () => {
    expect(modalityLabel('men', 'padel')).toBe('Caballeros');
    expect(modalityLabel('women', 'tennis')).toBe('Damas');
    expect(modalityLabel('mixed', 'pickleball')).toBe('Mixto');
  });

  it('names the modality of fútbol, básquet, vóley and hockey as Masculino and Femenino', () => {
    expect(modalityLabel('men', 'soccer')).toBe('Masculino');
    expect(modalityLabel('women', 'basketball')).toBe('Femenino');
    expect(modalityLabel('women', 'volleyball')).toBe('Femenino');
    expect(modalityLabel('men', 'hockey')).toBe('Masculino');
    expect(modalityLabel('mixed', 'soccer')).toBe('Mixto');
  });
});

describe('declarationWords', () => {
  it('asks a pádel player to be a caballero or a dama', () => {
    const words = declarationWords('padel');
    expect(words.genderLabel).toBe('Caballero / Dama');
    expect(words.genderOr).toBe('caballero o dama');
    expect(words.gender).toEqual({ men: 'Caballero', women: 'Dama' });
    expect(words.mixedPair).toBe('un caballero y una dama');
  });

  it('asks a fútbol player to be masculino or femenino', () => {
    const words = declarationWords('soccer');
    expect(words.genderLabel).toBe('Masculino / Femenino');
    expect(words.genderOr).toBe('masculino o femenino');
    expect(words.gender).toEqual({ men: 'Masculino', women: 'Femenino' });
    expect(words.playersOf).toEqual({ men: 'masculinos', women: 'femeninos' });
  });
});

describe('categoryTitle', () => {
  it('composes the modality and the level of pádel', () => {
    expect(categoryTitle({ ...CAT_OPEN, name: 'stored', modality: 'men', level: 8, suma: null }, 'padel')).toBe(
      'Caballeros 8va',
    );
    expect(categoryTitle({ ...CAT_OPEN, name: 'stored', modality: 'women', level: 5, suma: null }, 'padel')).toBe(
      'Damas 5ta',
    );
    expect(categoryTitle({ ...CAT_OPEN, name: 'stored', modality: 'mixed', level: 6, suma: null }, 'padel')).toBe(
      'Mixto 6ta',
    );
  });

  it('composes a Suma of pádel with its number', () => {
    expect(categoryTitle({ ...CAT_OPEN, name: 'stored', modality: 'mixed', level: null, suma: 15 }, 'padel')).toBe(
      'Mixto Suma 15',
    );
  });

  it('keeps the stored name for a category created before the traits', () => {
    expect(categoryTitle(CAT_OPEN, 'padel')).toBe('8va');
  });

  it('keeps the stored name when the modality is there but neither a level nor a Suma', () => {
    expect(categoryTitle({ ...CAT_OPEN, name: 'stored', modality: 'men', level: null, suma: null }, 'padel')).toBe(
      'stored',
    );
  });

  it('titles a fútbol category by its modality alone', () => {
    expect(categoryTitle({ ...CAT_OPEN, name: 'stored', modality: 'men', level: null, suma: null }, 'soccer')).toBe(
      'Masculino',
    );
    expect(categoryTitle({ ...CAT_OPEN, name: 'stored', modality: 'women', level: null, suma: null }, 'hockey')).toBe(
      'Femenino',
    );
  });

  it('titles a tenis category by its modality alone, with the racket words', () => {
    expect(categoryTitle({ ...CAT_OPEN, name: 'stored', modality: 'men', level: null, suma: null }, 'tennis')).toBe(
      'Caballeros',
    );
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

  it('titles a fútbol event by its modality alone', () => {
    expect(openPlayTitle({ sport: 'soccer', modality: 'women', level_min: null, level_max: null })).toBe('Femenino');
  });

  it('names a fútbol event without a modality by its sport', () => {
    expect(openPlayTitle({ sport: 'soccer', modality: null, level_min: null, level_max: null })).toBe(
      'Cancha abierta de fútbol',
    );
  });
});

describe('tournamentLabel', () => {
  it('names the sport in lower case', () => {
    expect(tournamentLabel('padel')).toBe('Torneo de pádel');
    expect(tournamentLabel('soccer')).toBe('Torneo de fútbol');
  });
});
