import { Panel } from '@/shared/components/common/Panel';
import { ES_AR } from '@/shared/i18n/es_AR';
import { MovementRow } from './MovementRow';
import type { CashMovement } from '@/shared/types/api.types';

const t = ES_AR;

interface MovementListProps {
  /** Newest first — callers pass the API's own order (`cashSessionsGet` returns it that way). */
  movements: CashMovement[];
  /** Omitted on a read-only view (a past, closed session's detail page). */
  onVoid?: ((movement: CashMovement) => void) | undefined;
}

export function MovementList({ movements, onVoid }: MovementListProps) {
  return (
    <Panel size="sm">
      <h2 className="text-text-tertiary mb-2 text-sm font-semibold tracking-wider uppercase">
        {t.cash.movementsTitle}
      </h2>
      {movements.length === 0 ? (
        <p className="text-text-tertiary py-6 text-center text-sm">{t.cash.noMovements}</p>
      ) : (
        <div>
          {movements.map((movement) => (
            <MovementRow key={movement.id} movement={movement} movements={movements} onVoid={onVoid} />
          ))}
        </div>
      )}
    </Panel>
  );
}
