import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/shared/components/common/AppDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CreateTournamentSteps } from './create-tournament-modal/CreateTournamentSteps';

const t = ES_AR.tournaments;

interface CreateTournamentModalProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
}

/** Mounted only while open, so every opening starts from a blank form. */
export function CreateTournamentModal({ open, onClose, complexId }: CreateTournamentModalProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-lg">
        {open && (
          <>
            <DialogHeader>
              <DialogTitle>{t.form.title}</DialogTitle>
              <DialogDescription>{t.form.description}</DialogDescription>
            </DialogHeader>
            <CreateTournamentSteps complexId={complexId} onClose={onClose} />
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
