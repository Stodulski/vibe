import { ArrowDown, ArrowUp } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { pairName } from '../lib/tournamentView';
import type { TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.draw;

interface SeedOrderEditorProps {
  /** The paid pairs, best seed first. */
  order: TournamentOwnerPair[];
  onMove: (pairId: string, direction: -1 | 1) => void;
}

/** The paid pairs in seed order, with buttons to move each one up or down. */
export function SeedOrderEditor({ order, onMove }: SeedOrderEditorProps) {
  return (
    <div className="space-y-2">
      <p className="text-text-tertiary text-xs">{t.seedOrderHint}</p>
      <ol aria-label={t.seedOrder} className="border-border-subtle divide-border-subtle divide-y rounded-xl border">
        {order.map((pair, index) => (
          <li key={pair.id} className="flex items-center gap-2 px-3 py-2">
            <span className="score-text text-text-tertiary w-5 text-xs">{index + 1}</span>
            <span className="text-text-primary min-w-0 flex-1 text-sm break-words">{pairName(pair)}</span>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={t.moveUp(pairName(pair))}
              disabled={index === 0}
              onClick={() => {
                onMove(pair.id, -1);
              }}
            >
              <ArrowUp className="size-4" aria-hidden="true" />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={t.moveDown(pairName(pair))}
              disabled={index === order.length - 1}
              onClick={() => {
                onMove(pair.id, 1);
              }}
            >
              <ArrowDown className="size-4" aria-hidden="true" />
            </Button>
          </li>
        ))}
      </ol>
    </div>
  );
}
