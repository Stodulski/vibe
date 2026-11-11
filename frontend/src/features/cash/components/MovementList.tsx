import { ES_AR } from '@/shared/i18n/es_AR';
import { MovementRow } from './MovementRow';
import { ledgerRows } from '../lib/movementLedger';
import type { CashMovement } from '@/shared/types/api.types';

const t = ES_AR;

interface MovementListProps {
  /** Newest first — callers pass the API's own order (`cashSessionsGet` returns it that way). */
  movements: CashMovement[];
  /** Omitted on a read-only view (a past, closed session's detail page). */
  onVoid?: ((movement: CashMovement) => void) | undefined;
}

export function MovementList({ movements, onVoid }: MovementListProps) {
  const rows = ledgerRows(movements);

  return (
    // No card: the list sits on the page, under the green till card.
    <section>
      <h2 className="text-text-primary mb-1 text-base font-semibold">{t.cash.movementsTitle}</h2>
      {rows.length === 0 ? (
        <p className="text-text-tertiary py-6 text-center text-sm">{t.cash.noMovements}</p>
      ) : (
        <div>
          {rows.map((row) => (
            <MovementRow key={row.movement.id} row={row} onVoid={onVoid} />
          ))}
        </div>
      )}
    </section>
  );
}
