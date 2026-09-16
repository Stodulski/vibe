import { useMemo } from 'react';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CardHeader } from './court-card/CardHeader';
import { PriceSummary } from './court-card/PriceSummary';
import { CardActions } from './court-card/CardActions';
import { getPriceRange } from './court-card/priceRange';
import { useCourtRowActions } from './court-card/useCourtRowActions';
import type { CourtWithPrices } from '@/shared/types/api.types';

const t = ES_AR;

interface CourtCardProps {
  court: CourtWithPrices;
  complexId: string;
  onEdit: (court: CourtWithPrices) => void;
  onPrices: (court: CourtWithPrices) => void;
}

export function CourtCard({ court, complexId, onEdit, onPrices }: CourtCardProps) {
  const priceRange = useMemo(() => getPriceRange(court.prices), [court.prices]);
  const { deleteOpen, setDeleteOpen, handleToggleActive, isTogglePending, handleDelete, isDeleting } =
    useCourtRowActions(court, complexId);

  return (
    <>
      <div
        className={cn(
          // `bg-bg-subtle` is what every other `rounded-2xl border` card in
          // the app sits on; this one had no background at all and was reading
          // as an outline drawn on the page. The price chips are `bg-elevated`,
          // one step up, so they still read as sitting on the card.
          'group bg-bg-subtle relative flex h-full flex-col overflow-hidden rounded-2xl border transition-colors',
          court.is_active ? 'border-border-subtle hover-lift glow-hover' : 'border-border-subtle/50 opacity-60',
        )}
      >
        <div className="flex-1 p-4 sm:p-5">
          <CardHeader court={court} onToggleActive={handleToggleActive} isTogglePending={isTogglePending} />
          <div className="mt-4">
            <PriceSummary {...priceRange} />
          </div>
        </div>

        <CardActions
          courtName={court.name}
          onEdit={() => {
            onEdit(court);
          }}
          onPrices={() => {
            onPrices(court);
          }}
          onDelete={() => {
            setDeleteOpen(true);
          }}
        />
      </div>

      <ConfirmDialog
        open={deleteOpen}
        onClose={() => {
          setDeleteOpen(false);
        }}
        onConfirm={handleDelete}
        title={t.courts.delete}
        description={t.courts.deleteConfirm}
        confirmLabel={t.common.delete}
        variant="destructive"
        isLoading={isDeleting}
      />
    </>
  );
}
