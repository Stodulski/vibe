import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@vibe/ui';
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
 * Opening hours, one markup at every width.
 *
 * The week is always in the HTML, so crawlers and no-JS readers get it. Below
 * `lg` the panel is collapsed by CSS on `data-state`, so the visitor opens the
 * week with one tap, and the trigger is the only heading. At `lg` the card has
 * room for the week at once: the trigger is hidden there, the panel is always
 * shown, and the screen-reader heading takes over the trigger's role.
 */
export function ComplexDetails({ schedules, selectedDate }: ComplexDetailsProps) {
  return (
    <Accordion type="single" collapsible className="border-border-subtle border-t lg:pt-5">
      <AccordionItem value="schedules" className="group">
        <div className="lg:hidden">
          <AccordionTrigger headingLevel={2}>{t.complex.schedules}</AccordionTrigger>
        </div>
        <h2 className="sr-only max-lg:hidden">{t.complex.schedules}</h2>
        <AccordionContent forceMount className="max-lg:group-data-[state=closed]:hidden lg:pb-0">
          <WeekScheduleList schedules={schedules} selectedDate={selectedDate} />
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  );
}
