import type { CourtEventBlock } from '@/shared/types/api.types';

/**
 * Cancelling or reading an event or a tournament is their pages' job: the grid
 * block opens it there, under the Eventos section. A literal path because
 * features do not import each other.
 */
export function eventBlockPath(block: CourtEventBlock): string {
  return 'match_id' in block ? `/events/tournaments/${block.tournament_id}` : `/events?event=${block.event_id}`;
}
