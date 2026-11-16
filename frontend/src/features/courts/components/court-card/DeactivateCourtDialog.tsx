import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { keepTogether } from '@/shared/lib/keepTogether';
import type { useCourtRowActions } from './useCourtRowActions';

const t = ES_AR;

interface DeactivateCourtDialogProps {
  courtName: string;
  actions: ReturnType<typeof useCourtRowActions>;
}

/** The question behind the "Activa" switch, shared by the card and the table row. */
export function DeactivateCourtDialog({ courtName, actions }: DeactivateCourtDialogProps) {
  return (
    <ConfirmDialog
      open={actions.deactivateOpen}
      onClose={() => {
        actions.setDeactivateOpen(false);
      }}
      onConfirm={actions.handleConfirmDeactivate}
      title={t.courts.deactivateTitle(keepTogether(courtName))}
      description={t.courts.deactivateConfirm}
      confirmLabel={t.courts.deactivate}
      variant="destructive"
      isLoading={actions.isTogglePending}
    />
  );
}
