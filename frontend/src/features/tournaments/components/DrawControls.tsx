import { useState } from 'react';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useGenerateDraw } from '../hooks/useTournamentChanges';
import { drawErrorMessage } from '../lib/tournamentErrors';
import type { CategoryView } from '../lib/categoryView';
import { SeedOrderEditor } from './SeedOrderEditor';
import type { TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.draw;

interface DrawControlsProps {
  complexId: string;
  tournamentId: string;
  categoryId: string;
  drawn: boolean;
  view: CategoryView;
}

/** Paid pairs best seed first: the existing seeds when drawn before, otherwise the order they signed up. */
function initialOrder(paid: TournamentOwnerPair[]): TournamentOwnerPair[] {
  return [...paid].sort((a, b) => (a.seed ?? Infinity) - (b.seed ?? Infinity));
}

function moved(order: TournamentOwnerPair[], pairId: string, direction: -1 | 1): TournamentOwnerPair[] {
  const from = order.findIndex((pair) => pair.id === pairId);
  const to = from + direction;
  const target = order[to];
  const current = order[from];
  if (!target || !current) return order;
  const next = [...order];
  next[from] = target;
  next[to] = current;
  return next;
}

/**
 * The draw: how many pairs paid, an optional seed order, and the button that
 * generates (or regenerates, until a result is loaded) the groups and bracket.
 */
export function DrawControls({ complexId, tournamentId, categoryId, drawn, view }: DrawControlsProps) {
  const generate = useGenerateDraw(complexId, tournamentId, categoryId);
  const [order, setOrder] = useState<TournamentOwnerPair[] | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const enough = view.paidPairs.length >= 2;

  return (
    <div className="border-border-subtle space-y-3 rounded-2xl border p-3.5">
      <h3 className="text-text-primary text-sm font-semibold">{t.title}</h3>
      <p className="text-text-tertiary text-xs">{drawn ? t.regenerateHint : t.generateHint}</p>
      {!enough && <p className="text-warning-text text-xs">{t.needTwo}</p>}
      {view.locked && <p className="text-text-tertiary text-xs">{t.locked}</p>}
      {order && (
        <SeedOrderEditor
          order={order}
          onMove={(pairId, direction) => {
            setOrder((prev) => (prev ? moved(prev, pairId, direction) : prev));
          }}
        />
      )}
      <DrawError error={generate.error} />
      <DrawButtons
        drawn={drawn}
        showSeedToggle={!view.locked && enough}
        seedOn={!!order}
        disabled={!enough || view.locked || generate.isPending}
        onToggleSeed={() => {
          setOrder(order ? null : initialOrder(view.paidPairs));
        }}
        onGenerate={() => {
          setConfirmOpen(true);
        }}
      />
      <ConfirmDialog
        open={confirmOpen}
        onClose={() => {
          setConfirmOpen(false);
        }}
        onConfirm={() => {
          generate.mutate(order?.map((pair) => pair.id) ?? [], {
            onSuccess: () => {
              setConfirmOpen(false);
              setOrder(null);
            },
            onError: () => {
              setConfirmOpen(false);
            },
          });
        }}
        title={t.confirmTitle}
        description={drawn ? t.confirmRegenerate : t.confirmDescription}
        confirmLabel={t.confirmLabel}
        cancelLabel={t.back}
        isLoading={generate.isPending}
      />
    </div>
  );
}

interface DrawButtonsProps {
  drawn: boolean;
  showSeedToggle: boolean;
  seedOn: boolean;
  disabled: boolean;
  onToggleSeed: () => void;
  onGenerate: () => void;
}

function DrawButtons({ drawn, showSeedToggle, seedOn, disabled, onToggleSeed, onGenerate }: DrawButtonsProps) {
  return (
    <div className="flex flex-col gap-2 sm:flex-row">
      {showSeedToggle && (
        <Button type="button" variant="outline" className="rounded-xl" onClick={onToggleSeed}>
          {seedOn ? t.seedOrderOff : t.seedOrderOn}
        </Button>
      )}
      <Button type="button" className="rounded-xl" disabled={disabled} onClick={onGenerate}>
        {drawn ? t.regenerate : t.generate}
      </Button>
    </div>
  );
}

function DrawError({ error }: { error: unknown }) {
  if (!error) return null;
  return (
    <p role="alert" className="text-error-text text-sm">
      {drawErrorMessage(error)}
    </p>
  );
}
