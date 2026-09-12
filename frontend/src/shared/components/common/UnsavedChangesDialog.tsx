import { ES_AR } from '@/shared/i18n/es_AR';
import { ConfirmDialog } from './ConfirmDialog';
import type { UnsavedChangesBlocker } from '@/shared/hooks/useUnsavedChangesBlocker';

const t = ES_AR;

/**
 * The confirmation a blocked navigation waits on — see
 * `useUnsavedChangesBlocker`, whose return value is this component's only prop.
 *
 * Destructive, because the button that closes it destroys work: the thing being
 * confirmed is discarding what was typed, not merely moving elsewhere.
 */
export function UnsavedChangesDialog({ blocker }: { blocker: UnsavedChangesBlocker }) {
  return (
    <ConfirmDialog
      open={blocker.isBlocked}
      onClose={blocker.stay}
      onConfirm={blocker.leave}
      title={t.common.unsavedChangesTitle}
      description={t.common.unsavedChangesDescription}
      confirmLabel={t.common.unsavedChangesConfirm}
      variant="destructive"
    />
  );
}
