import { TimeSlotButton } from './TimeSlotButton';
import { ContinueAction } from './ContinueAction';
import { TIME_GROUPS } from './constants';
import type { TimeOption } from './timeOptions';
import type { TimeGroup } from './slotMath';

interface TimeGroupGridProps {
  groupKey: TimeGroup;
  options: TimeOption[];
  typesVary: boolean;
  selectedStartTime: string | null;
  /** Whether the settled selection belongs to this group, so the action shows here. */
  showContinue: boolean;
  onSelectTime: (option: TimeOption) => void;
  onContinue: () => void;
}

/**
 * One part of the day — morning, afternoon, evening — as a grid of hours.
 *
 * The court question is not asked here. When an hour needs one, the selector
 * replaces every grid with the court cards for that hour, so the two are
 * never on screen together.
 */
export function TimeGroupGrid({
  groupKey,
  options,
  typesVary,
  selectedStartTime,
  showContinue,
  onSelectTime,
  onContinue,
}: TimeGroupGridProps) {
  const group = TIME_GROUPS.find((g) => g.key === groupKey);
  if (!group) return null; // type-level only: keys come from TIME_GROUPS
  const Icon = group.icon;

  return (
    <div>
      <div className="mb-2 flex items-center gap-1.5">
        <Icon className="size-3.5 text-text-tertiary" />
        <span className="text-xs font-semibold uppercase tracking-wide text-text-tertiary">{group.label}</span>
      </div>
      <div
        className="grid grid-cols-3 gap-2 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-8"
        role="group"
        aria-label={group.label}
      >
        {options.map((option) => (
          <TimeSlotButton
            key={option.startTime}
            option={option}
            isSelected={option.startTime === selectedStartTime}
            typesVary={typesVary}
            onSelect={onSelectTime}
          />
        ))}
      </div>
      {showContinue && <ContinueAction onContinue={onContinue} />}
    </div>
  );
}
