import { cn } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';

export type ComplexSection = 'book' | 'events';

const t = BOOKING_MESSAGES.publicBooking;

const OPTIONS: readonly { value: ComplexSection; label: string }[] = [
  { value: 'book', label: t.sectionBook },
  { value: 'events', label: t.sectionEvents },
];

/**
 * The two halves of a complex page that has events: booking a court, or its events.
 * A segmented control: one rounded track, the chosen half filled in the brand color.
 */
export function SectionSwitch({
  section,
  onChange,
}: {
  section: ComplexSection;
  onChange: (section: ComplexSection) => void;
}) {
  return (
    <div
      role="group"
      aria-label={t.sectionSwitchLabel}
      className="bg-bg-subtle border-border-subtle grid grid-cols-2 gap-1 rounded-full border p-1"
    >
      {OPTIONS.map((option) => (
        <button
          key={option.value}
          type="button"
          aria-pressed={option.value === section}
          onClick={() => {
            onChange(option.value);
          }}
          className={cn(
            'flex h-11 cursor-pointer items-center justify-center rounded-full px-4 text-sm font-semibold transition-colors duration-200',
            option.value === section
              ? 'bg-primary-500 text-text-inverse shadow-sm'
              : 'text-text-secondary hover:text-text-primary',
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
