import { AlertCircle } from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/shared/components/common/AppDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { submitHandler } from '@/shared/lib/form';
import { categoryTitle } from '@/shared/lib/traitLabels';
import { ScheduleFields } from './schedule-dialog/ScheduleFields';
import { useScheduleForm } from './schedule-dialog/useScheduleForm';
import type { CourtWithPrices, Tournament, TournamentCategory, TournamentMatch } from '@/shared/types/api.types';

const t = ES_AR.tournaments.schedule;

interface ScheduleMatchDialogProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  tournament: Tournament;
  category: TournamentCategory;
  match: TournamentMatch | null;
  courts: CourtWithPrices[];
}

/** Gives a match a court, a day and a time. Mounted only while open, so every opening starts from the match's own hours. */
export function ScheduleMatchDialog({ open, onClose, match, ...rest }: ScheduleMatchDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-lg">
        {open && match && <ScheduleBody match={match} onClose={onClose} {...rest} />}
      </DialogContent>
    </Dialog>
  );
}

type BodyProps = Omit<ScheduleMatchDialogProps, 'open' | 'match'> & { match: TournamentMatch };

function ConflictNotice({ conflict, message }: { conflict: string | null; message: string | null }) {
  if (!message) return null;
  return (
    <div role="alert" className="border-error-border bg-error-bg flex items-start gap-2.5 rounded-xl border p-3">
      <AlertCircle className="text-error-icon mt-0.5 size-4 shrink-0" aria-hidden="true" />
      <div className="min-w-0 text-sm">
        {conflict ? (
          <>
            <p className="text-error-text font-medium">{t.conflict.title}</p>
            <p className="text-text-secondary mt-0.5 break-words">{conflict}</p>
            <p className="text-text-tertiary mt-1 text-xs">{t.conflict.hint}</p>
          </>
        ) : (
          <p className="text-error-text break-words">{message}</p>
        )}
      </div>
    </div>
  );
}

function ScheduleBody({ onClose, complexId, tournament, category, match, courts }: BodyProps) {
  const { form, onSubmit, isPending, conflict, errorMessage } = useScheduleForm({
    complexId,
    tournament,
    category,
    match,
    onDone: onClose,
  });

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t.title}</DialogTitle>
        <DialogDescription>{categoryTitle(category)}</DialogDescription>
      </DialogHeader>
      <form onSubmit={submitHandler(form.handleSubmit, onSubmit)} className="space-y-4" noValidate>
        <ScheduleFields
          form={form}
          courts={courts}
          defaultDuration={category.match_duration_minutes}
          firstDay={tournament.start_date}
          lastDay={tournament.end_date}
        />
        <ConflictNotice conflict={conflict} message={errorMessage} />
        <SectionFooter onCancel={onClose} cancelDisabled={isPending} submitLabel={t.submit} pending={isPending} />
      </form>
    </>
  );
}
