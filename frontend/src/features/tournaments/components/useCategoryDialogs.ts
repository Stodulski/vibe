import { useState } from 'react';

/** Which dialog of the category is open and for what: a match to schedule or load a result for, a pair to swap, the cancel. */
export function useCategoryDialogs() {
  const [scheduleMatchId, setScheduleMatchId] = useState<string | null>(null);
  const [resultMatchId, setResultMatchId] = useState<string | null>(null);
  const [swapPairId, setSwapPairId] = useState<string | null>(null);
  const [cancelOpen, setCancelOpen] = useState(false);
  return {
    scheduleMatchId,
    setScheduleMatchId,
    resultMatchId,
    setResultMatchId,
    swapPairId,
    setSwapPairId,
    cancelOpen,
    setCancelOpen,
  };
}

export type CategoryDialogs = ReturnType<typeof useCategoryDialogs>;
