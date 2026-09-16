import { TableCell, TableRow } from '@/shared/components/ui/table';
import { Switch } from '@/shared/components/ui/switch';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useCourtRowActions } from '../court-card/useCourtRowActions';
import { CourtDescriptionCell } from './CourtDescriptionCell';
import { CourtPriceCell } from './CourtPriceCell';
import { CourtRowActionsMenu } from './CourtRowActionsMenu';
import type { CourtWithPrices } from '@/shared/types/api.types';

const t = ES_AR;

interface CourtTableRowProps {
  court: CourtWithPrices;
  complexId: string;
  onEdit: (court: CourtWithPrices) => void;
  onPrices: (court: CourtWithPrices) => void;
}

/**
 * One court, as a dense table row — the `xl` reading of the same
 * `CourtCard`. The toggle and the delete confirmation come from
 * `useCourtRowActions`, the same hook the card uses, so the two renderings
 * can never disagree about what either action does.
 */
export function CourtTableRow({ court, complexId, onEdit, onPrices }: CourtTableRowProps) {
  const { deleteOpen, setDeleteOpen, handleToggleActive, isTogglePending, handleDelete, isDeleting } =
    useCourtRowActions(court, complexId);

  return (
    <>
      <TableRow className={cn('border-border-subtle hover:bg-bg-elevated', !court.is_active && 'opacity-60')}>
        <TableCell className="max-w-56">
          <p className="text-text-primary truncate text-sm font-semibold" title={court.name}>
            {court.name}
          </p>
          <p className="text-text-tertiary truncate text-xs">{t.courts.sportTypes[court.sport]}</p>
        </TableCell>

        <TableCell className="text-text-secondary text-sm whitespace-nowrap">
          {t.courts.courtTypes[court.court_type]}
        </TableCell>

        <TableCell className="max-w-64">
          <CourtDescriptionCell description={court.description} />
        </TableCell>

        <TableCell className="text-right">
          <CourtPriceCell prices={court.prices} />
        </TableCell>

        <TableCell>
          <Switch
            checked={court.is_active}
            onCheckedChange={handleToggleActive}
            disabled={isTogglePending}
            aria-label={`${court.name}: ${court.is_active ? t.courts.active : t.courts.inactive}`}
          />
        </TableCell>

        <TableCell className="text-right">
          <CourtRowActionsMenu
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
        </TableCell>
      </TableRow>

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
