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
  /** libres: one entry per sport with a free hour. Empty on liberado. */
  sections: SportHours[];
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

/** "Hoy", "Mañana", or the weekday and day, e.g. "lunes 23". */
export function dayLabel(date: string, today: string): string {
  const offset = daysBetween(today, date);
  if (offset === 0) return 'Hoy';
  if (offset === 1) return 'Mañana';
  const weekday = new Intl.DateTimeFormat('es-AR', { weekday: 'long', timeZone: 'UTC' }).format(
    new Date(`${date}T12:00:00Z`),
  );
  return `${weekday} ${Number(date.slice(8))}`;
}

function freeHeadline(date: string, today: string): string {
  const day = dayLabel(date, today);
  if (day === 'Hoy' || day === 'Mañana') return `Turnos libres ${day.toLowerCase()}`;
  return `Turnos libres el ${day}`;
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

/**
 * The story's words, or null when there is nothing true to say: a day with no free hour, or a
 * freed slot that is taken again, is not on the grid, or names a court the complex lacks.
 */
export function storyText(
  facts: ShareFacts,
  grid: StoryAvailability,
  slug: string,
  request: StoryRequest,
  today: string,
  labels: Readonly<Record<string, string>>,
): StoryText | null {
  if (!grid.is_open) return null;
  const base = { name: facts.name.trim(), link: `${STOREFRONT_ORIGIN}${complexPath(slug)}`.replace(/^https:\/\//, '') };

  if (request.piece === 'libres') {
    const sections = freeSections(grid, labels);
    if (sections.length === 0) return null;
    return { ...base, headline: freeHeadline(request.date, today), sections };
  }

  const court = grid.courts.find((c) => c.court_id === request.courtId);
  const free = court?.slots.some((s) => s.start_time === request.time && s.available);
  if (!court || !free || !request.time) return null;
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
