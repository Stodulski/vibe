/**
 * The words of the Instagram stories a complex publishes: "Turnos libres" lists the hours still
 * bookable on a day, "Se liberó un turno" announces one slot. Both are drawn only from the public
 * availability grid, so a story never shows an hour that cannot be booked right now, and nobody can
 * make the endpoint draw words of their own. No QR: the story shows the complex's address only.
 * Pure, so node:test can run it; the route passes the sport labels and today's venue date in.
 */
import type { ShareFacts } from '../complex-share.ts';
import { complexPath } from '../hub.ts';
import { STOREFRONT_ORIGIN } from '../storefront.ts';

export const STORY_PIECES = ['libres', 'liberado'] as const;
export type StoryPiece = (typeof STORY_PIECES)[number];

export interface StoryRequest {
  piece: StoryPiece;
  /** The venue date, YYYY-MM-DD. */
  date: string;
  /** liberado only: the slot's start, HH:MM, and its court. */
  time?: string;
  courtId?: string;
  /** The slot length the grid is read for; the API's default when absent. */
  duration?: number;
}

/** The part of the public availability answer a story reads. */
export interface StoryAvailability {
  date: string;
  is_open: boolean;
  courts: ReadonlyArray<{
    court_id: string;
    court_name: string;
    sport: string;
    slots: ReadonlyArray<{ start_time: string; start_min: number; available: boolean }>;
  }>;
}

export interface SportHours {
  sport: string;
  /** The first free hours of the day, in day order. */
  hours: string[];
  /** Free hours that did not fit. */
  more: number;
}

export interface StoryText {
  name: string;
  headline: string;
  /** The complex page without the scheme: the only address a story shows. */
  link: string;
  /** libres for today or tomorrow: the date under the headline, e.g. "Jueves 20". */
  dateLine?: string;
  /** libres: one entry per sport with a free hour. Empty on liberado and on the empty preview. */
  sections: SportHours[];
  /** The preview of a day with nothing free: what the card says instead of hours. */
  empty?: string;
  /** liberado: the freed slot. sport is empty when the court name already says it. */
  slot?: { time: string; day: string; court: string; sport: string };
}

/** At most this many sports fit a story. */
export const MAX_SPORTS = 3;
/** Hours shown per sport, by how many sports share the story: three rows, two, or one each. */
const HOURS_PER_SPORT: Readonly<Record<number, number>> = { 1: 12, 2: 8, 3: 4 };

const DATE_RX = /^\d{4}-\d{2}-\d{2}$/;
const TIME_RX = /^([01]\d|2[0-3]):[0-5]\d$/;

/** Reads the piece from the path and the slot from the query; null when anything is malformed. */
export function parseStoryRequest(piece: string, query: URLSearchParams, today: string): StoryRequest | null {
  if (!(STORY_PIECES as readonly string[]).includes(piece)) return null;
  const date = query.get('fecha') ?? today;
  if (!DATE_RX.test(date)) return null;
  const request: StoryRequest = { piece: piece as StoryPiece, date };
  const duration = query.get('duracion');
  if (duration !== null) {
    if (!/^\d{2,3}$/.test(duration)) return null;
    request.duration = Number(duration);
  }
  if (request.piece === 'liberado') {
    const time = query.get('hora') ?? '';
    const courtId = query.get('cancha') ?? '';
    if (!TIME_RX.test(time) || courtId === '') return null;
    request.time = time;
    request.courtId = courtId;
  }
  return request;
}

/** Days between two YYYY-MM-DD dates, read as calendar dates. */
function daysBetween(from: string, to: string): number {
  return Math.round((Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`)) / 86_400_000);
}

/** The weekday and day of the month in es-AR, e.g. "lunes 23". */
function weekdayDay(date: string): string {
  const weekday = new Intl.DateTimeFormat('es-AR', { weekday: 'long', timeZone: 'UTC' }).format(
    new Date(`${date}T12:00:00Z`),
  );
  return `${weekday} ${Number(date.slice(8))}`;
}

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/** The day with its date, since a story stays up 24 hours: "Hoy, jueves 20", "Mañana, viernes 21", "lunes 23". */
export function dayLabel(date: string, today: string): string {
  const offset = daysBetween(today, date);
  if (offset === 0) return `Hoy, ${weekdayDay(date)}`;
  if (offset === 1) return `Mañana, ${weekdayDay(date)}`;
  return weekdayDay(date);
}

/** The libres headline and, for today and tomorrow, the date line under it. */
function freeHeading(date: string, today: string): { headline: string; dateLine?: string } {
  const offset = daysBetween(today, date);
  if (offset === 0) return { headline: 'Turnos libres hoy', dateLine: capitalize(weekdayDay(date)) };
  if (offset === 1) return { headline: 'Turnos libres mañana', dateLine: capitalize(weekdayDay(date)) };
  return { headline: `Turnos libres el ${weekdayDay(date)}` };
}

function emptyLine(date: string, today: string): string {
  const offset = daysBetween(today, date);
  const day = offset === 0 ? 'Hoy' : offset === 1 ? 'Mañana' : `El ${weekdayDay(date)}`;
  return `${day} no quedan turnos libres`;
}

/** The calendar day before a YYYY-MM-DD date. */
export function previousDate(date: string): string {
  return new Date(Date.parse(`${date}T00:00:00Z`) - 86_400_000).toISOString().slice(0, 10);
}

function minutesOf(time: string): number {
  return Number(time.slice(0, 2)) * 60 + Number(time.slice(3, 5));
}

/**
 * The freed slot's court and whether it is free on one operating day's grid. An overnight venue
 * files the hours after midnight under the previous operating day with start_min >= 1440, so the
 * 00:30 of date C is start_min 30 on grid(C) or start_min 1470 on grid(C-1); grid(C)'s own 1470 is
 * the night after and never matches.
 */
function freedSlotOn(
  grid: StoryAvailability | undefined,
  request: StoryRequest,
  offset: number,
): StoryAvailability['courts'][number] | null {
  if (!grid?.is_open || !request.time) return null;
  const minutes = minutesOf(request.time) + offset;
  const court = grid.courts.find((c) => c.court_id === request.courtId);
  return court?.slots.some((s) => s.start_min === minutes && s.available) ? court : null;
}

/**
 * True when a freed slot is not free on its own day, so the previous operating day must be read.
 * Never before today: the public grid refuses past dates, and an after-midnight hour of yesterday's
 * operating day cannot be booked from it anyway.
 */
export function needsPreviousNight(grid: StoryAvailability, request: StoryRequest, today: string): boolean {
  return request.piece === 'liberado' && previousDate(request.date) >= today && freedSlotOn(grid, request, 0) === null;
}

function freeSections(grid: StoryAvailability, labels: Readonly<Record<string, string>>): SportHours[] {
  const bySport = new Map<string, Map<string, number>>();
  for (const court of grid.courts) {
    const sport = labels[court.sport] ?? court.sport;
    for (const slot of court.slots) {
      if (!slot.available) continue;
      const hours = bySport.get(sport) ?? new Map<string, number>();
      hours.set(slot.start_time, slot.start_min);
      bySport.set(sport, hours);
    }
  }
  const sports = [...bySport.entries()].slice(0, MAX_SPORTS);
  const cap = HOURS_PER_SPORT[sports.length] ?? 4;
  return sports.map(([sport, hours]) => {
    const ordered = [...hours.entries()].sort((a, b) => a[1] - b[1]).map(([time]) => time);
    return { sport, hours: ordered.slice(0, cap), more: Math.max(0, ordered.length - cap) };
  });
}

function baseText(facts: ShareFacts, slug: string): Pick<StoryText, 'name' | 'link'> {
  return { name: facts.name.trim(), link: `${STOREFRONT_ORIGIN}${complexPath(slug)}`.replace(/^https:\/\//, '') };
}

/**
 * The story's words, or null when there is nothing true to say: a day with no free hour, or a
 * freed slot that is taken again, is not on the grid, or names a court the complex lacks. A freed
 * slot after midnight is also looked for on `previous`, the operating day before the request's date.
 */
export function storyText(
  facts: ShareFacts,
  grid: StoryAvailability,
  slug: string,
  request: StoryRequest,
  today: string,
  labels: Readonly<Record<string, string>>,
  previous?: StoryAvailability,
): StoryText | null {
  const base = baseText(facts, slug);

  if (request.piece === 'libres') {
    if (!grid.is_open) return null;
    const sections = freeSections(grid, labels);
    if (sections.length === 0) return null;
    return { ...base, ...freeHeading(request.date, today), sections };
  }

  const court = freedSlotOn(grid, request, 0) ?? freedSlotOn(previous, request, 1440);
  if (!court || !request.time) return null;
  const courtName = court.court_name.trim();
  const sport = labels[court.sport] ?? court.sport;
  return {
    ...base,
    headline: 'Se liberó un turno',
    sections: [],
    slot: {
      time: request.time,
      day: dayLabel(request.date, today),
      court: courtName,
      // "Pádel 2 · Pádel" says the sport twice: a court named after its sport shows the name alone.
      sport: courtName.toLowerCase().includes(sport.toLowerCase()) ? '' : sport,
    },
  };
}

/** The panel's preview of a libres day with nothing free: the day, and a card that says so. */
export function emptyFreeText(facts: ShareFacts, slug: string, request: StoryRequest, today: string): StoryText {
  // "Turnos libres hoy" over "Hoy no quedan turnos libres" contradicts itself: the headline is the day.
  return {
    ...baseText(facts, slug),
    headline: capitalize(dayLabel(request.date, today)),
    sections: [],
    empty: emptyLine(request.date, today),
  };
}
