import { TimeGroupGrid } from './TimeGroupGrid';
import { TIME_GROUPS } from './constants';
import type { TimeOption } from './timeOptions';
import type { TimeGroup } from './slotMath';
import type { SelectedSlot } from './types';

interface TimeGroupListProps {
  grouped: Map<TimeGroup, TimeOption[]>;
  typesVary: boolean;
  severalCourts: boolean;
  selected: SelectedSlot | null;
  onSelectTime: (option: TimeOption) => void;
}

/** The parts of the day, in order, skipping any the venue has no hours in. */
export function TimeGroupList({ grouped, typesVary, severalCourts, selected, onSelectTime }: TimeGroupListProps) {
  return (
    <>
      {TIME_GROUPS.map(({ key }) => {
        const options = grouped.get(key);
        if (!options || options.length === 0) return null;

        return (
          <TimeGroupGrid
            key={key}
            groupKey={key}
            options={options}
            typesVary={typesVary}
            severalCourts={severalCourts}
            selectedStartTime={selected?.slot.start_time ?? null}
            onSelectTime={onSelectTime}
          />
        );
      })}
    </>
  );
}
