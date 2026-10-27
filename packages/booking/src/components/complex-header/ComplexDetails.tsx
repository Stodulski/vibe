import { BOOKING_MESSAGES } from '../../messages';
import type { Schedule } from '../../api/types';
import { WeekScheduleList } from './WeekScheduleList';

const t = BOOKING_MESSAGES;

interface ComplexDetailsProps {
  schedules: Schedule[];
  /** Forwarded to `WeekScheduleList` — see its own doc comment. */
  selectedDate?: Date | undefined;
}

/**
 * A block of the club page. No visible heading: its content says what it is,
 * and a rule above does the separating. The heading stays for screen readers,
 * which cannot see icons or the shape of a list.
 *
 * Shared with `ComplexLower`, which holds the services and the map.
 */
export function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="border-border-subtle border-t pt-5">
      <h2 className="sr-only">{title}</h2>
      {children}
    </section>
  );
}

/**
 * Opening hours, the whole week open at every width. A player checks the hours
 * before choosing a club, so they are not behind a tap on a phone either.
 */
export function ComplexDetails({ schedules, selectedDate }: ComplexDetailsProps) {
  return (
    <Section title={t.complex.schedules}>
      <WeekScheduleList schedules={schedules} selectedDate={selectedDate} />
    </Section>
  );
}
