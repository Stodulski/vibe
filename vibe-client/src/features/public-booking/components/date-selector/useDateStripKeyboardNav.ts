import { useRef } from 'react';
import type { Schedule } from '@/shared/types/api.types';
import { isClosedDay } from './dayUtils';

/**
 * Roving tabindex for the date strip: only the selected date sits in the Tab
 * order (`DateButton` sets `tabIndex={isSelected ? 0 : -1}`), so Tab always
 * leaves the strip in one press regardless of how many days have loaded.
 * Arrow Left/Right/Home/End move both focus and the selection between dates,
 * skipping closed days — this is what a screen reader / keyboard user needs
 * instead of the old flat list of dozens of tab stops.
 */
export function useDateStripKeyboardNav(days: Date[], schedules: Schedule[], onDateSelect: (date: Date) => void) {
  const buttonRefs = useRef<(HTMLButtonElement | null)[]>([]);

  const findNextEnabledIndex = (from: number, dir: 1 | -1) => {
    let idx = from + dir;
    while (idx >= 0 && idx < days.length) {
      const date = days[idx];
      if (date && !isClosedDay(date, schedules)) return idx;
      idx += dir;
    }
    return null;
  };

  const findEdgeEnabledIndex = (dir: 1 | -1) => {
    let idx = dir === 1 ? days.length - 1 : 0;
    while (idx >= 0 && idx < days.length) {
      const date = days[idx];
      if (date && !isClosedDay(date, schedules)) return idx;
      idx += dir === 1 ? -1 : 1;
    }
    return null;
  };

  const moveTo = (index: number | null) => {
    const date = index === null ? undefined : days[index];
    if (date === undefined || index === null) return;
    onDateSelect(date);
    buttonRefs.current[index]?.focus();
  };

  const handleKeyDown = (event: React.KeyboardEvent, currentIndex: number) => {
    switch (event.key) {
      case 'ArrowRight':
        event.preventDefault();
        moveTo(findNextEnabledIndex(currentIndex, 1));
        break;
      case 'ArrowLeft':
        event.preventDefault();
        moveTo(findNextEnabledIndex(currentIndex, -1));
        break;
      case 'Home':
        event.preventDefault();
        moveTo(findEdgeEnabledIndex(-1));
        break;
      case 'End':
        event.preventDefault();
        moveTo(findEdgeEnabledIndex(1));
        break;
      default:
        break;
    }
  };

  return { buttonRefs, handleKeyDown };
}
