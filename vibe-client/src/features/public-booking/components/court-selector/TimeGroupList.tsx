import { TimeGroupGrid } from './TimeGroupGrid';
import { TIME_GROUPS } from './constants';
import type { TimeOption } from './timeOptions';
import type { TimeGroup } from './slotMath';
import type { SelectedSlot } from './types';

interface TimeGroupListProps {
  grouped: Map<TimeGroup, TimeOption[]>;
  typesVary: boolean;
  selected: SelectedSlot | null;
  onSelectTime: (option: TimeOption) => void;
  onContinue: () => void;
}

/** The parts of the day, in order, skipping any the venue has no hours in. */
export function TimeGroupList({ grouped, typesVary, selected, onSelectTime, onContinue }: TimeGroupListProps) {
  return (
    <>
      {TIME_GROUPS.map(({ key }) => {
        const options = grouped.get(key);
        if (!options || options.length === 0) return null;

        // The action belongs to the group holding the settled choice, so it
        // lands under the hour that was tapped instead of at the foot of the
        // page.
        const showContinue = !!selected && options.some((o) => o.startTime === selected.slot.start_time);

        return (
          <TimeGroupGrid
            key={key}
            groupKey={key}
            options={options}
            typesVary={typesVary}
            selectedStartTime={selected?.slot.start_time ?? null}
            showContinue={showContinue}
            onSelectTime={onSelectTime}
            onContinue={onContinue}
          />
        );
      })}
    </>
  );
}
