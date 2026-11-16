import type { TournamentOwnerMatch } from '@/shared/types/api.types';

/** What a category's matches have collected for the court and what is still to collect, in centavos. */
export interface CourtFeeSummary {
  collected: number;
  pending: number;
}

/**
 * Collected is the sum of the charges at the amount each was made; pending is
 * what the pairs that can still be charged owe at the current fee.
 */
export function courtFeeSummary(matches: TournamentOwnerMatch[]): CourtFeeSummary {
  let collected = 0;
  let pending = 0;
  for (const match of matches) {
    for (const side of match.court_fee) {
      if (side.charged) collected += side.amount;
      else if (side.chargeable) pending += side.amount;
    }
  }
  return { collected, pending };
}
