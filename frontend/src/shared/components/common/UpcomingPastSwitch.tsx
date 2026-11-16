import { cn } from '@/shared/lib/utils';
import { Button } from '@/shared/components/ui/button';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR.common;

export type UpcomingPast = 'upcoming' | 'past';

const OPTIONS: { value: UpcomingPast; label: string }[] = [
  { value: 'upcoming', label: t.upcoming },
  { value: 'past', label: t.past },
];

/**
 * Próximos / Pasados, as two toggle buttons that share the width on a phone.
 * One switch for every list that splits by time (canchas abiertas, torneos), so
 * the same choice is never drawn two ways. 40px tall: the buttons are the
 * control, not a decoration, and 32px is under a comfortable touch target.
 */
export function UpcomingPastSwitch({
  value,
  onChange,
}: {
  value: UpcomingPast;
  onChange: (value: UpcomingPast) => void;
}) {
  return (
    <div className="flex gap-2">
      {OPTIONS.map((option) => (
        <Button
          key={option.value}
          type="button"
          variant={value === option.value ? 'default' : 'outline'}
          aria-pressed={value === option.value}
          className={cn(
            'h-10 flex-1 rounded-xl border sm:flex-none sm:px-5',
            value === option.value && 'border-transparent',
          )}
          onClick={() => {
            onChange(option.value);
          }}
        >
          {option.label}
        </Button>
      ))}
    </div>
  );
}

/** Loading shape of `UpcomingPastSwitch`: two buttons of the same height and gap. */
export function UpcomingPastSwitchSkeleton() {
  return (
    <div className="flex gap-2" aria-hidden="true">
      <Skeleton className="h-10 flex-1 rounded-xl sm:w-24 sm:flex-none" />
      <Skeleton className="h-10 flex-1 rounded-xl sm:w-20 sm:flex-none" />
    </div>
  );
}
