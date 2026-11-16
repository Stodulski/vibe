import { createContext, useContext } from 'react';
import { PADEL_SPORT } from './traitLabels';

/**
 * The sport of the tournament on screen, so that a player's or a category's labels in a deep
 * component read in that sport's words (S15). Padel when no screen provides one.
 */
export const SportContext = createContext<string>(PADEL_SPORT);

export function useSport(): string {
  return useContext(SportContext);
}
