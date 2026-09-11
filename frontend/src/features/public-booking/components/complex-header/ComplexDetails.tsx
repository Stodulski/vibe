import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/shared/components/ui/accordion';
import { useMediaQuery } from '@/shared/hooks/useMediaQuery';
import { ES_AR } from '@/shared/i18n/es_AR';
import { orderedAmenities } from '@/shared/lib/amenities';
import type { Schedule } from '@/shared/types/api.types';
import { WeekScheduleList } from './WeekScheduleList';
import { AmenityList } from './AmenityList';

const t = ES_AR;

/** Tailwind's `lg`, where the header becomes a card with room for both lists open. */
const OPEN_SECTIONS = '(min-width: 1024px)';

interface ComplexDetailsProps {
  schedules: Schedule[];
  amenities: readonly string[];
  /** Forwarded to `WeekScheduleList` — see its own doc comment. */
  selectedDate?: Date | undefined;
}

/**
 * A block of the open layout. No visible heading: seven days with hours and
 * a list of amenities with icons say what they are, and a rule between them
 * does the separating. The heading stays for screen readers, which cannot
 * see the icons or the shape of the list.
 */
function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="border-t border-border-subtle pt-5">
      <h2 className="sr-only">{title}</h2>
      {children}
    </section>
  );
}

/**
 * Opening hours and services — open in the column, collapsed in the stack.
 *
 * Beside the identity block there is room for both at once, and a control
 * that hides content already on screen is a tap charged for nothing. Once the
 * header stacks into one column that stops being true: the two lists together
 * push the bookable hours off the screen, so they become an accordion whose
 * `type="single"` lets only one stand open at a time.
 *
 * The switch is a real branch, not two trees with one hidden by CSS — hiding
 * would leave a screen reader reading every heading twice and finding a
 * button that does nothing.
 *
 * A venue with no services listed gets no services section at all, rather
 * than a heading that opens onto nothing.
 */
export function ComplexDetails({ schedules, amenities, selectedDate }: ComplexDetailsProps) {
  const openSections = useMediaQuery(OPEN_SECTIONS);
  const hasAmenities = orderedAmenities(amenities).length > 0;

  if (openSections) {
    return (
      <div className="space-y-5">
        <Section title={t.complex.schedules}>
          <WeekScheduleList schedules={schedules} selectedDate={selectedDate} />
        </Section>
        {hasAmenities && (
          <Section title={t.complex.amenitiesSection}>
            <AmenityList amenities={amenities} />
          </Section>
        )}
      </div>
    );
  }

  return (
    <Accordion type="single" collapsible defaultValue="schedules" className="border-t border-border-subtle">
      <AccordionItem value="schedules">
        <AccordionTrigger>{t.complex.schedules}</AccordionTrigger>
        <AccordionContent>
          <WeekScheduleList schedules={schedules} selectedDate={selectedDate} />
        </AccordionContent>
      </AccordionItem>

      {hasAmenities && (
        <AccordionItem value="amenities">
          <AccordionTrigger>{t.complex.amenitiesSection}</AccordionTrigger>
          <AccordionContent>
            <AmenityList amenities={amenities} />
          </AccordionContent>
        </AccordionItem>
      )}
    </Accordion>
  );
}
