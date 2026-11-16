import { useState } from 'react';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { Panel } from '@/shared/components/common/Panel';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useGenerateDraw } from '../hooks/useTournamentChanges';
import { drawErrorMessage } from '../lib/tournamentErrors';
import type { CategoryView } from '../lib/categoryView';
import { SeedOrderEditor } from './SeedOrderEditor';
import type { TournamentOwnerCategory, TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.draw;

interface DrawControlsProps {
  complexId: string;
  tournamentId: string;
  category: TournamentOwnerCategory;
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

/** What the draw does now: repeatable, or its first run, which names the zones only when the category has them. */
function drawHint(category: TournamentOwnerCategory, drawn: boolean): string {
  if (drawn) return t.regenerateHint;
  return category.format === 'groups' ? t.generateHint : t.generateHintElimination;
}

/**
 * The draw: how many pairs paid, an optional seed order, and the button that
 * generates (or regenerates, until a result is loaded) the groups and bracket.
 * Once a result is loaded there is no draw to offer, so nothing renders here.
 */
export function DrawControls({ complexId, tournamentId, category, drawn, view }: DrawControlsProps) {
  const generate = useGenerateDraw(complexId, tournamentId, category.id);
  const [order, setOrder] = useState<TournamentOwnerPair[] | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [refusal, setRefusal] = useState<string | null>(null);
  const enough = view.paidPairs.length >= 2;
  if (view.locked) return null;

  return (
    <>
      <Panel size="md" mobile="card" className="space-y-4">
        <div className="space-y-1">
          <h3 className="text-text-primary text-sm font-semibold">{t.title}</h3>
          <p className="text-text-tertiary text-xs">{drawHint(category, drawn)}</p>
        </div>
        {order && (
          <SeedOrderEditor
            order={order}
            onMove={(pairId, direction) => {
              setOrder((prev) => (prev ? moved(prev, pairId, direction) : prev));
            }}
          />
        )}
        <DrawError error={refusal ?? generate.error} category={category} pairs={view.paidPairs.length} />
        <DrawButtons
          drawn={drawn}
          showSeedToggle={enough}
          seedOn={!!order}
          disabled={generate.isPending}
          onToggleSeed={() => {
            setOrder(order ? null : initialOrder(view.paidPairs));
          }}
          onGenerate={() => {
            // The button stays enabled: a disabled one says nothing about why. The reason is told here, on pressing it.
            const reason = enough ? null : t.errors.notEnough;
            setRefusal(reason);
            if (!reason) setConfirmOpen(true);
          }}
        />
      </Panel>
      <DrawConfirm
        open={confirmOpen}
        drawn={drawn}
        isPending={generate.isPending}
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
      />
    </>
  );
}

interface DrawConfirmProps {
  open: boolean;
  drawn: boolean;
  isPending: boolean;
  onClose: () => void;
  onConfirm: () => void;
}

/** The confirmation before a draw: the first run closes the registration, a repeat says what it undoes. */
function DrawConfirm({ open, drawn, isPending, onClose, onConfirm }: DrawConfirmProps) {
  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={onConfirm}
      title={t.confirmTitle}
      description={drawn ? t.confirmRegenerate : t.confirmDescription}
      confirmLabel={t.confirmLabel}
      isLoading={isPending}
    />
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

/** The most important action first: the draw, then the optional seed order beside it. */
function DrawButtons({ drawn, showSeedToggle, seedOn, disabled, onToggleSeed, onGenerate }: DrawButtonsProps) {
  return (
    <div className="flex flex-col gap-2 sm:flex-row">
      <Button type="button" className="w-full rounded-xl sm:w-auto" disabled={disabled} onClick={onGenerate}>
        {drawn ? t.regenerate : t.generate}
      </Button>
      {showSeedToggle && (
        <Button type="button" variant="outline" className="w-full rounded-xl sm:w-auto" onClick={onToggleSeed}>
          {seedOn ? t.seedOrderOff : t.seedOrderOn}
        </Button>
      )}
    </div>
  );
}

function DrawError({ error, category, pairs }: { error: unknown; category: TournamentOwnerCategory; pairs: number }) {
  if (!error) return null;
  // A refusal is already a sentence; anything else is a request error to translate.
  if (typeof error === 'string') {
    return (
      <p role="alert" className="text-error-text text-sm">
        {error}
      </p>
    );
  }
  return (
    <p role="alert" className="text-error-text text-sm">
      {drawErrorMessage(error, {
        pairs,
        groupSize: category.group_size,
        qualifiers: category.qualifiers_per_group,
      })}
    </p>
  );
}
