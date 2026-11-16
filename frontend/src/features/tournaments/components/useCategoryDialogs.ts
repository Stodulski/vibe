import { useState } from 'react';
import type { CourtFeeTarget } from '../hooks/useTournamentChanges';

/**
 * Which dialog of the category is open and for what: a match to schedule or load
 * a result for, a pair to swap, the cancel, the court fee to edit, and the pair
 * (a side of a match) to charge the court fee or to undo it for.
 */
export function useCategoryDialogs() {
  const [scheduleMatchId, setScheduleMatchId] = useState<string | null>(null);
  const [resultMatchId, setResultMatchId] = useState<string | null>(null);
  const [swapPairId, setSwapPairId] = useState<string | null>(null);
  const [cancelOpen, setCancelOpen] = useState(false);
  const [feeOpen, setFeeOpen] = useState(false);
  const [chargeTarget, setChargeTarget] = useState<CourtFeeTarget | null>(null);
  const [undoTarget, setUndoTarget] = useState<CourtFeeTarget | null>(null);
  return {
    scheduleMatchId,
    setScheduleMatchId,
    resultMatchId,
    setResultMatchId,
    swapPairId,
    setSwapPairId,
    cancelOpen,
    setCancelOpen,
    feeOpen,
    setFeeOpen,
    chargeTarget,
    setChargeTarget,
    undoTarget,
    setUndoTarget,
  };
}

export type CategoryDialogs = ReturnType<typeof useCategoryDialogs>;
