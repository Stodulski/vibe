import { CalendarPlus, Trophy, type LucideIcon } from 'lucide-react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/shared/components/common/AppDialog';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR.events.typePicker;

export type CreateEventKind = 'openPlay' | 'tournament';

interface EventTypePickerProps {
  open: boolean;
  onClose: () => void;
  onPick: (kind: CreateEventKind) => void;
}

const OPTIONS: { kind: CreateEventKind; icon: LucideIcon; label: string; hint: string }[] = [
  { kind: 'openPlay', icon: CalendarPlus, label: t.openPlay, hint: t.openPlayHint },
  { kind: 'tournament', icon: Trophy, label: t.tournament, hint: t.tournamentHint },
];

/** Asks which kind to create before its form opens: a cancha abierta or a torneo. */
export function EventTypePicker({ open, onClose, onPick }: EventTypePickerProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t.title}</DialogTitle>
        </DialogHeader>
        <div className="grid gap-3">
          {OPTIONS.map(({ kind, icon: Icon, label, hint }) => (
            <button
              key={kind}
              type="button"
              onClick={() => {
                onPick(kind);
              }}
              className="border-border-subtle bg-bg-subtle hover:border-border-default focus-self flex min-h-24 items-center gap-4 rounded-2xl border p-4 text-left transition-colors"
            >
              <Icon className="text-text-secondary size-6 shrink-0" aria-hidden="true" />
              <span className="flex min-w-0 flex-col gap-1">
                <span className="text-text-primary text-base font-semibold">{label}</span>
                <span className="text-text-secondary text-sm">{hint}</span>
              </span>
            </button>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}
