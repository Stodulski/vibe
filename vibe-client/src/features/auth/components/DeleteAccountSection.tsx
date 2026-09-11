import { DangerActionsMenu } from '@/shared/components/common/DangerActionsMenu';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useState } from 'react';
import { useDeleteAccount } from '../hooks/useDeleteAccount';

const t = ES_AR;

export function DeleteAccountSection() {
  const [open, setOpen] = useState(false);

  const mutation = useDeleteAccount();

  return (
    <>
      <DangerActionsMenu
        label={t.profile.deleteAccount}
        onSelect={() => {
          setOpen(true);
        }}
      />

      <ConfirmDialog
        open={open}
        onClose={() => {
          setOpen(false);
        }}
        onConfirm={() => {
          mutation.mutate(undefined, {
            onError: () => {
              setOpen(false);
            },
          });
        }}
        title={t.profile.deleteAccount}
        description={t.profile.deleteAccountConfirm}
        confirmLabel={t.profile.deleteAccountLabel}
        variant="destructive"
        isLoading={mutation.isPending}
      />
    </>
  );
}
