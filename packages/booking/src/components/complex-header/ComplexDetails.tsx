import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@vibe/ui';
import { useMediaQuery } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import type { Schedule } from '../../api/types';
import { WeekScheduleList } from './WeekScheduleList';

const t = BOOKING_MESSAGES;

/** Tailwind's `lg`, where the header becomes a card with room for the week open. */
const OPEN_SECTIONS = '(min-width: 1024px)';

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
 * Opening hours — a section in the open card at `lg`, an accordion below it.
 *
 * Beside the identity block there is room for the week at once, and a control
 * that hides content already on screen is a tap charged for nothing. Once the
 * header stacks into one column that stops being true: the slot picker follows
 * the card directly, so the hours start closed and open only when the visitor
 * asks to read the week.
 *
 * The switch is a real branch, not two trees with one hidden by CSS — hiding
 * would leave a screen reader reading every heading twice and finding a
 * button that does nothing.
 */
export function ComplexDetails({ schedules, selectedDate }: ComplexDetailsProps) {
  const openSections = useMediaQuery(OPEN_SECTIONS);

  if (openSections) {
    return (
      <Section title={t.complex.schedules}>
        <WeekScheduleList schedules={schedules} selectedDate={selectedDate} />
      </Section>
    );
  }

  return (
    <Accordion type="single" collapsible className="border-border-subtle border-t">
      <AccordionItem value="schedules">
        <AccordionTrigger headingLevel={2}>{t.complex.schedules}</AccordionTrigger>
        <AccordionContent>
          <WeekScheduleList schedules={schedules} selectedDate={selectedDate} />
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  );
}
