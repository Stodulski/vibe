/**
 * The words of the Instagram stories: which hours a "free today" story lists and when a "slot
 * freed" story may be drawn at all. A story never shows an hour that is not bookable right now.
 * Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  dayLabel,
  emptyFreeText,
  needsPreviousNight,
  parseStoryRequest,
  storyText,
  type StoryAvailability,
} from '../src/lib/story/story-text.ts';

const facts = { name: 'La Zona F5', city: 'Caleta Olivia', province: 'Santa Cruz', sports: ['Pádel'] };
const LABELS = { padel: 'Pádel', soccer: 'Fútbol' };
const TODAY = '2026-11-19';

function slot(start: string, startMin: number, available: boolean, duration = 90) {
  return { start_time: start, end_time: '', start_min: startMin, duration_minutes: duration, price: 1, available };
}

function availability(): StoryAvailability {
  return {
    date: TODAY,
    is_open: true,
    courts: [
      {
        court_id: 'c1',
        court_name: 'Cancha 1',
        sport: 'padel',
        slots: [slot('21:00', 1260, true), slot('18:00', 1080, true), slot('19:30', 1170, false)],
      },
      {
        court_id: 'c2',
        court_name: 'Cancha 2',
        sport: 'padel',
        slots: [slot('18:00', 1080, true), slot('19:30', 1170, true)],
      },
      { court_id: 'c3', court_name: 'F5', sport: 'soccer', slots: [slot('20:00', 1200, false)] },
    ],
  };
}

const libres = { piece: 'libres', date: TODAY } as const;

test('a free-hours story lists each sport once, its free hours unique and in day order, and skips taken ones', () => {
  const text = storyText(facts, availability(), 'la-zona-f5', libres, TODAY, LABELS);
  assert.ok(text);
  assert.deepEqual(text.sections, [{ sport: 'Pádel', hours: ['18:00', '19:30', '21:00'], more: 0 }]);
  assert.equal(text.headline, 'Turnos libres hoy');
  assert.equal(text.dateLine, 'Jueves 19');
  assert.equal(text.link, 'vibe.com.ar/c/la-zona-f5');
});

test('a day with no free hour has no story', () => {
  const full = availability();
  for (const court of full.courts) for (const s of court.slots) s.available = false;
  assert.equal(storyText(facts, full, 'la-zona-f5', libres, TODAY, LABELS), null);
  assert.equal(storyText(facts, { ...availability(), is_open: false }, 'la-zona-f5', libres, TODAY, LABELS), null);
});

test('a sport with more free hours than fit shows the first ones and says how many more', () => {
  const many = availability();
  many.courts[0].slots = Array.from({ length: 13 }, (_, i) => slot(`${String(9 + i).padStart(2, '0')}:00`, 540 + i * 60, true));
  const text = storyText(facts, many, 'la-zona-f5', libres, TODAY, LABELS);
  assert.ok(text);
  const padel = text.sections[0];
  assert.equal(padel.hours.length + padel.more, 14, 'every free hour is either shown or counted');
  assert.equal(padel.hours[0], '09:00');
  assert.ok(padel.more > 0);
});

test('the headline says tomorrow, or the weekday and day, for other dates', () => {
  const tomorrow = storyText(facts, availability(), 'la-zona-f5', { piece: 'libres', date: '2026-11-20' }, TODAY, LABELS);
  assert.equal(tomorrow?.headline, 'Turnos libres mañana');
  assert.equal(tomorrow?.dateLine, 'Viernes 20');
  const later = storyText(facts, availability(), 'la-zona-f5', { piece: 'libres', date: '2026-11-23' }, TODAY, LABELS);
  assert.equal(later?.headline, 'Turnos libres el lunes 23');
  assert.equal(later?.dateLine, undefined, 'the headline already carries the date');
});

test('a freed-slot story names the court, the sport and the hour while that slot is still free', () => {
  const text = storyText(
    facts,
    availability(),
    'la-zona-f5',
    { piece: 'liberado', date: TODAY, time: '19:30', courtId: 'c2' },
    TODAY,
    LABELS,
  );
  assert.ok(text);
  assert.equal(text.headline, 'Se liberó un turno');
  assert.deepEqual(text.slot, { time: '19:30', day: 'Hoy, jueves 19', court: 'Cancha 2', sport: 'Pádel' });
});

test('a freed-slot story does not repeat the sport when the court is named after it', () => {
  const grid = availability();
  grid.courts[1].court_name = 'Pádel 2';
  const text = storyText(
    facts,
    grid,
    'la-zona-f5',
    { piece: 'liberado', date: TODAY, time: '19:30', courtId: 'c2' },
    TODAY,
    LABELS,
  );
  assert.equal(text?.slot?.court, 'Pádel 2');
  assert.equal(text?.slot?.sport, '');
});

test('a freed-slot story for a slot taken again, unknown, or of another court is not drawn', () => {
  const ask = (time: string, courtId: string) =>
    storyText(facts, availability(), 'la-zona-f5', { piece: 'liberado', date: TODAY, time, courtId }, TODAY, LABELS);
  assert.equal(ask('19:30', 'c1'), null, 'taken on that court');
  assert.equal(ask('17:00', 'c2'), null, 'not on the grid');
  assert.equal(ask('19:30', 'nope'), null, 'unknown court');
});

test('the request is read from the path and the query, with today as the default date', () => {
  assert.deepEqual(parseStoryRequest('libres', new URLSearchParams(), TODAY), { piece: 'libres', date: TODAY });
  assert.deepEqual(
    parseStoryRequest('liberado', new URLSearchParams('fecha=2026-11-20&hora=19:30&cancha=c2&duracion=90'), TODAY),
    { piece: 'liberado', date: '2026-11-20', time: '19:30', courtId: 'c2', duration: 90 },
  );
});

test('an unknown piece, a bad date or hour, or a freed slot without its court is refused', () => {
  assert.equal(parseStoryRequest('reservar', new URLSearchParams(), TODAY), null);
  assert.equal(parseStoryRequest('libres', new URLSearchParams('fecha=19/11/2026'), TODAY), null);
  assert.equal(parseStoryRequest('liberado', new URLSearchParams('hora=25:00&cancha=c2'), TODAY), null);
  assert.equal(parseStoryRequest('liberado', new URLSearchParams('hora=19:30'), TODAY), null);
  assert.equal(parseStoryRequest('libres', new URLSearchParams('duracion=abc'), TODAY), null);
});

test('the day label carries the date: today, tomorrow, and other days by weekday', () => {
  assert.equal(dayLabel(TODAY, TODAY), 'Hoy, jueves 19');
  assert.equal(dayLabel('2026-11-20', TODAY), 'Mañana, viernes 20');
  assert.equal(dayLabel('2026-11-23', TODAY), 'lunes 23');
});

/** An overnight venue: operating day 2026-11-19 runs past midnight, filed as start_min >= 1440. */
function night(onPrevious: boolean | null, sameDayLateFree = true) {
  const day = availability();
  day.date = '2026-11-20';
  // The 20th's own after-midnight hour is the night of the 21st: never the 00:30 of the 20th.
  day.courts[0].slots = [slot('18:00', 1080, true), slot('00:30', 1470, sameDayLateFree)];
  const previous = availability();
  previous.courts[0].slots = onPrevious === null ? [slot('18:00', 1080, true)] : [slot('00:30', 1470, onPrevious)];
  return { day, previous };
}

const afterMidnight = { piece: 'liberado', date: '2026-11-20', time: '00:30', courtId: 'c1' } as const;

test('a freed slot after midnight is found on the previous operating day, not on the next night', () => {
  const { day, previous } = night(true);
  assert.equal(needsPreviousNight(day, afterMidnight, TODAY), true);
  assert.equal(storyText(facts, day, 'la-zona-f5', afterMidnight, TODAY, LABELS), null, 'the 20th grid alone holds the wrong night');
  const text = storyText(facts, day, 'la-zona-f5', afterMidnight, TODAY, LABELS, previous);
  assert.equal(text?.slot?.time, '00:30');
  assert.equal(text?.slot?.day, 'Mañana, viernes 20');
});

test('a freed slot after midnight taken again on the previous night is not drawn', () => {
  const { day, previous } = night(false);
  assert.equal(storyText(facts, day, 'la-zona-f5', afterMidnight, TODAY, LABELS, previous), null);
  const missing = night(null);
  assert.equal(storyText(facts, missing.day, 'la-zona-f5', afterMidnight, TODAY, LABELS, missing.previous), null);
});

test('a slot that starts on its own operating day needs no previous night', () => {
  const day = availability();
  day.courts[0].slots = [slot('00:30', 30, true)];
  assert.equal(needsPreviousNight(day, afterMidnight, TODAY), false);
  assert.equal(storyText(facts, day, 'la-zona-f5', afterMidnight, TODAY, LABELS)?.slot?.time, '00:30');
  assert.equal(needsPreviousNight(availability(), libres, TODAY), false);
});

test('yesterday is never read: the public grid refuses past dates', () => {
  const { day } = night(false);
  assert.equal(needsPreviousNight(day, { ...afterMidnight, date: TODAY }, TODAY), false);
});

test('the empty preview says no free hours are left, worded for the day', () => {
  const today = emptyFreeText(facts, 'la-zona-f5', libres, TODAY);
  assert.equal(today.empty, 'Hoy no quedan turnos libres');
  assert.equal(today.headline, 'Hoy, jueves 19', 'no "Turnos libres" headline over an empty day');
  assert.equal(emptyFreeText(facts, 'la-zona-f5', { piece: 'libres', date: '2026-11-20' }, TODAY).empty, 'Mañana no quedan turnos libres');
  const later = emptyFreeText(facts, 'la-zona-f5', { piece: 'libres', date: '2026-11-23' }, TODAY);
  assert.equal(later.empty, 'El lunes 23 no quedan turnos libres');
  assert.deepEqual(later.sections, []);
});
