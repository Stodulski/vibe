import { formatPrice } from '@vibe/ui';
import type { PublicOpenPlayEvent } from '../../api/types';
import { BOOKING_MESSAGES } from '../../messages';
import { courtNamesLine } from '../../lib/courtNames';
import { keepTogether } from '../../lib/keepTogether';
import { publicEventPath } from '../../lib/publicPaths';
import { openPlayAudience, openPlayLabel } from '../../lib/traitLabels';
import { longDayLabel } from '../agenda/agendaModel';
import type { AgendaDetail } from '../agenda/AgendaDetailDialog';
import { eventHasStarted, eventTimeRange, isOpenForSignUp } from './eventFormat';

const t = BOOKING_MESSAGES.events;

/** One open-play event on the agenda: its label, who it is for, and the day it starts. */
export interface OpenPlayRow {
  event: PublicOpenPlayEvent;
  /** The day the event starts (`YYYY-MM-DD`). */
  date: string;
  /** "Cancha abierta de pádel" */
  label: string;
  /** "Damas · 6ta a 8va", "Masculino" or "Abierta a todos". */
  title: string;
}

/**
 * The open-play rows: one per event open for sign-up with a place left and not
 * yet started, sorted by day and start time. A full or cancelled event is left out.
 */
export function openPlayRows(events: readonly PublicOpenPlayEvent[]): OpenPlayRow[] {
  return events
    .filter((event) => isOpenForSignUp(event) && !eventHasStarted(event))
    .sort(
      (a, b) => a.date.localeCompare(b.date) || a.start_time.localeCompare(b.start_time) || a.id.localeCompare(b.id),
    )
    .map((event) => ({
      event,
      date: event.date,
      label: openPlayLabel(event.sport),
      title: openPlayAudience(event),
    }));
}

/**
 * The detail of an open-play event: its day and time, what a person pays, its
 * courts, who may play, and the sign-up.
 */
export function openPlayDetail(row: OpenPlayRow, slug: string): AgendaDetail {
  const { event } = row;
  const courts = event.court_names.map(keepTogether);
  return {
    title: row.label,
    when: `${longDayLabel(row.date)} · ${eventTimeRange(event)}`,
    facts: [
      { term: t.detailPriceLabel, value: `${formatPrice(event.price)} ${t.perPerson}` },
      ...(courts.length > 0 ? [{ term: t.courtsLabel, value: courtNamesLine(courts) }] : []),
      { term: t.detailRequirementLabel, value: row.title },
    ],
    signUp: { href: publicEventPath(slug, event.id), label: t.signUp },
  };
}
