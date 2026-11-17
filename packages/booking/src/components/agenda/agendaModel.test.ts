import { describe, expect, it } from 'vitest';
import { dayChips, longDayLabel, matchesAgendaFilter, modalityOptions, spotsNotice } from './agendaModel';

const SPOTS_COPY = { lastSpots: 'Últimos lugares', spotsLeft: (count: number) => `Quedan ${String(count)} lugares` };

describe('longDayLabel', () => {
  it('writes the weekday, the day and the month of a date', () => {
    expect(longDayLabel('2099-12-06')).toBe('Domingo 06/12');
    expect(longDayLabel('2099-12-05')).toBe('Sábado 05/12');
  });
});

describe('dayChips', () => {
  it('offers each listed day once, in order, for any row with a date', () => {
    expect(dayChips([{ date: '2099-12-06' }, { date: '2099-12-05' }, { date: '2099-12-06' }])).toEqual([
      { date: '2099-12-05', label: 'Sáb 5 dic' },
      { date: '2099-12-06', label: 'Dom 6 dic' },
    ]);
  });
});

describe('spotsNotice', () => {
  it('counts the places while there are more than two', () => {
    expect(spotsNotice(7, SPOTS_COPY)).toEqual({ text: 'Quedan 7 lugares', last: false });
  });

  it('says the last places, highlighted, when two or fewer remain', () => {
    expect(spotsNotice(2, SPOTS_COPY)).toEqual({ text: 'Últimos lugares', last: true });
    expect(spotsNotice(1, SPOTS_COPY)).toEqual({ text: 'Últimos lugares', last: true });
  });
});

describe('matchesAgendaFilter', () => {
  it('keeps every row for all modalities and all days', () => {
    expect(matchesAgendaFilter('men', '2099-12-05', { modality: 'all', day: null })).toBe(true);
    expect(matchesAgendaFilter(null, '2099-12-05', { modality: 'all', day: null })).toBe(true);
  });

  it('keeps the rows of one modality, and a row without one only under all', () => {
    expect(matchesAgendaFilter('women', '2099-12-05', { modality: 'women', day: null })).toBe(true);
    expect(matchesAgendaFilter('men', '2099-12-05', { modality: 'women', day: null })).toBe(false);
    expect(matchesAgendaFilter(null, '2099-12-05', { modality: 'women', day: null })).toBe(false);
  });

  it('keeps the rows of one day and combines it with a modality', () => {
    expect(matchesAgendaFilter('men', '2099-12-06', { modality: 'all', day: '2099-12-06' })).toBe(true);
    expect(matchesAgendaFilter('men', '2099-12-05', { modality: 'all', day: '2099-12-06' })).toBe(false);
    expect(matchesAgendaFilter('men', '2099-12-06', { modality: 'women', day: '2099-12-06' })).toBe(false);
  });
});

describe('modalityOptions', () => {
  it('names the modalities of pádel as Caballeros and Damas, in the order men, women, mixed', () => {
    expect(
      modalityOptions([
        { modality: 'women', sport: 'padel' },
        { modality: 'men', sport: 'padel' },
      ]),
    ).toEqual([
      { value: 'men', label: 'Caballeros' },
      { value: 'women', label: 'Damas' },
    ]);
  });

  it('names the modalities of fútbol as Masculino, Femenino and Mixto', () => {
    expect(
      modalityOptions([
        { modality: 'women', sport: 'soccer' },
        { modality: 'mixed', sport: 'soccer' },
      ]),
    ).toEqual([
      { value: 'women', label: 'Femenino' },
      { value: 'mixed', label: 'Mixto' },
    ]);
  });

  it('offers only the modalities the rows have, and none for rows without one', () => {
    expect(modalityOptions([{ modality: null, sport: 'padel' }])).toEqual([]);
  });

  it('joins the words of two sports that name the same modality differently', () => {
    expect(
      modalityOptions([
        { modality: 'men', sport: 'padel' },
        { modality: 'men', sport: 'soccer' },
      ]),
    ).toEqual([{ value: 'men', label: 'Caballeros / Masculino' }]);
  });
});
