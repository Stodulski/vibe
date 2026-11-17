import { describe, it, expect } from 'vitest';
import {
  categoryStartLabel,
  categoryTitle,
  levelLabel,
  modalityLabel,
  openPlayTitle,
  playerTraitsLabel,
  tournamentTitle,
} from './traitLabels';

describe('modalityLabel', () => {
  it('reads each modality as the owner says it', () => {
    expect(modalityLabel('men')).toBe('Caballeros');
    expect(modalityLabel('women')).toBe('Damas');
    expect(modalityLabel('mixed')).toBe('Mixto');
  });
});

describe('modalityLabel by sport', () => {
  it('says caballeros and damas for padel, tennis and pickleball', () => {
    for (const sport of ['padel', 'tennis', 'pickleball']) {
      expect(modalityLabel('men', sport)).toBe('Caballeros');
      expect(modalityLabel('women', sport)).toBe('Damas');
      expect(modalityLabel('mixed', sport)).toBe('Mixto');
    }
  });

  it('says masculino and femenino for football, basketball, volleyball and hockey', () => {
    for (const sport of ['soccer', 'basketball', 'volleyball', 'hockey']) {
      expect(modalityLabel('men', sport)).toBe('Masculino');
      expect(modalityLabel('women', sport)).toBe('Femenino');
      expect(modalityLabel('mixed', sport)).toBe('Mixto');
    }
  });
});

describe('levelLabel', () => {
  it('reads 1 to 9 as ordinals, with the owner’s spellings for 7 and 8', () => {
    expect(['1ra', '2da', '3ra', '4ta', '5ta', '6ta', '7ma', '8va', '9na'].map((_, i) => levelLabel(i + 1))).toEqual([
      '1ra',
      '2da',
      '3ra',
      '4ta',
      '5ta',
      '6ta',
      '7ma',
      '8va',
      '9na',
    ]);
  });
});

describe('categoryTitle', () => {
  it('composes a level category from its modality: "Caballeros 8va", "Damas 5ta", "Mixto 6ta"', () => {
    expect(categoryTitle({ modality: 'men', level: 8, suma: null })).toBe('Caballeros 8va');
    expect(categoryTitle({ modality: 'women', level: 5, suma: null })).toBe('Damas 5ta');
    expect(categoryTitle({ modality: 'mixed', level: 6, suma: null })).toBe('Mixto 6ta');
  });

  it('composes a suma category: "Mixto Suma 15"', () => {
    expect(categoryTitle({ modality: 'mixed', level: null, suma: 15 })).toBe('Mixto Suma 15');
  });

  it('names a category of another sport by its modality alone: "Masculino", "Damas"', () => {
    expect(categoryTitle({ sport: 'soccer', modality: 'men', level: null, suma: null })).toBe('Masculino');
    expect(categoryTitle({ sport: 'tennis', modality: 'women', level: null, suma: null })).toBe('Damas');
  });

  it('falls back to the stored name for a legacy category without modality', () => {
    expect(categoryTitle({ modality: null, level: null, suma: null, name: '6ta' })).toBe('6ta');
    expect(categoryTitle({ name: 'Libre' })).toBe('Libre');
  });

  it('gives an empty title for a draft with neither modality nor name', () => {
    expect(categoryTitle({ modality: undefined, level: undefined, suma: undefined })).toBe('');
  });
});

describe('openPlayTitle', () => {
  it('names a cancha abierta from its sport alone when it has no traits (legacy)', () => {
    expect(openPlayTitle({ sport: 'padel', modality: null, level_min: null, level_max: null })).toBe(
      'Cancha abierta de pádel',
    );
  });

  it('adds the modality and the level range: "· 6ta a 8va"', () => {
    expect(openPlayTitle({ sport: 'padel', modality: 'men', level_min: 6, level_max: 8 })).toBe(
      'Cancha abierta de pádel · Caballeros · 6ta a 8va',
    );
  });

  it('says a single level as "solo 4ta"', () => {
    expect(openPlayTitle({ sport: 'padel', modality: 'women', level_min: 4, level_max: 4 })).toBe(
      'Cancha abierta de pádel · Damas · solo 4ta',
    );
  });

  it('adds only the modality when there is no range', () => {
    expect(openPlayTitle({ sport: 'padel', modality: 'mixed', level_min: null, level_max: null })).toBe(
      'Cancha abierta de pádel · Mixto',
    );
  });

  it('lowercases the sport inside the title', () => {
    expect(openPlayTitle({ sport: 'soccer', modality: null })).toBe('Cancha abierta de fútbol');
  });
});

describe('tournamentTitle', () => {
  it('names a tournament from its sport', () => {
    expect(tournamentTitle('padel')).toBe('Torneo de pádel');
    expect(tournamentTitle('soccer')).toBe('Torneo de fútbol');
  });
});

describe('categoryStartLabel', () => {
  it('reads the day and the time: "Sáb 21 nov · 18:00" (21 Nov 2026 is a Saturday)', () => {
    expect(categoryStartLabel('2026-11-21', '18:00')).toBe('Sáb 21 nov · 18:00');
  });

  it('reads the day alone when there is no time yet', () => {
    expect(categoryStartLabel('2026-11-21', null)).toBe('Sáb 21 nov');
  });

  it('has no label for a legacy category without a day', () => {
    expect(categoryStartLabel(null, null)).toBeNull();
    expect(categoryStartLabel('', '18:00')).toBeNull();
  });
});

describe('playerTraitsLabel', () => {
  it('reads the declared gender and category: "Caballero · 7ma"', () => {
    expect(playerTraitsLabel({ gender: 'men', level: 7 })).toBe('Caballero · 7ma');
  });

  it('reads the gender of a masculine sport as masculino or femenino', () => {
    expect(playerTraitsLabel({ sport: 'soccer', gender: 'women', level: null })).toBe('Femenino');
    expect(playerTraitsLabel({ sport: 'padel', gender: 'women', level: null })).toBe('Dama');
  });

  it('shows only what was declared', () => {
    expect(playerTraitsLabel({ gender: 'women', level: null })).toBe('Dama');
    expect(playerTraitsLabel({ gender: null, level: 3 })).toBe('3ra');
  });

  it('has nothing to show for a sign-up made before the traits', () => {
    expect(playerTraitsLabel({ gender: null, level: null })).toBeNull();
  });
});
