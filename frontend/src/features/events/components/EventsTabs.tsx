import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { EventsTab } from '../pages/events/useEventsPage';

const t = ES_AR.events;

const TABS: { value: EventsTab; label: string }[] = [
  { value: 'upcoming', label: t.upcoming },
  { value: 'past', label: t.past },
];

/** Upcoming or past, as two toggle buttons that share the width on a phone. */
export function EventsTabs({ active, onChange }: { active: EventsTab; onChange: (tab: EventsTab) => void }) {
  return (
    <div className="flex gap-2">
      {TABS.map((tab) => (
        <Button
          key={tab.value}
          type="button"
          size="sm"
          variant={active === tab.value ? 'default' : 'outline'}
          aria-pressed={active === tab.value}
          className="flex-1 rounded-xl sm:flex-none sm:px-5"
          onClick={() => {
            onChange(tab.value);
          }}
        >
          {tab.label}
        </Button>
      ))}
    </div>
  );
}
