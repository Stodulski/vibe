import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/shared/components/common/AppDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { slotLabel } from '../lib/tournamentView';
import { SegmentedTabs } from './SegmentedTabs';
import { SetRows } from './result-dialog/SetRows';
import { SidePicker } from './result-dialog/SidePicker';
import { useResultForm, type ResultMode } from './result-dialog/useResultForm';
import type { TournamentCategory, TournamentMatch, TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.result;

const MODES: { value: ResultMode; label: string }[] = [
  { value: 'winner', label: t.modeWinner },
  { value: 'sets', label: t.modeSets },
  { value: 'walkover', label: t.modeWalkover },
];

interface ResultDialogProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  tournamentId: string;
  category: TournamentCategory;
  match: TournamentMatch | null;
  pairs: Map<string, TournamentOwnerPair>;
}

/** Loads or corrects a match's result: only the winner, the sets (as many as the category plays) or a walkover. Mounted only while open. */
export function ResultDialog({ open, onClose, match, pairs, ...rest }: ResultDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        {open && match && <ResultBody match={match} pairs={pairs} onClose={onClose} {...rest} />}
      </DialogContent>
    </Dialog>
  );
}

type BodyProps = Omit<ResultDialogProps, 'open' | 'match'> & { match: TournamentMatch };

function ResultBody({ onClose, complexId, tournamentId, category, match, pairs }: BodyProps) {
  const form = useResultForm({ complexId, tournamentId, category, match, onDone: onClose });
  const names: [string, string] = [slotLabel(match, 1, pairs, new Map()), slotLabel(match, 2, pairs, new Map())];

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t.title}</DialogTitle>
        <DialogDescription>
          {names[0]} {ES_AR.tournaments.matches.versus} {names[1]}
        </DialogDescription>
      </DialogHeader>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          form.submit();
        }}
        className="space-y-4"
        noValidate
      >
        <SegmentedTabs items={MODES} active={form.mode} onChange={form.setMode} ariaLabel={t.modeLabel} wrap />
        {form.mode === 'sets' && <SetRows rows={form.rows} format={form.format} names={names} onChange={form.setRow} />}
        {form.mode === 'winner' && (
          <SidePicker
            names={names}
            value={form.winner}
            onChange={form.setWinner}
            question={t.winnerQuestion}
            hint={t.winnerHint}
          />
        )}
        {form.mode === 'walkover' && (
          <SidePicker
            names={names}
            value={form.loser}
            onChange={form.setLoser}
            question={t.walkoverQuestion}
            hint={t.walkoverHint}
          />
        )}
        {form.error && (
          <p role="alert" className="text-error-text text-sm">
            {form.error}
          </p>
        )}
        <SectionFooter
          onCancel={onClose}
          cancelDisabled={form.isPending}
          submitLabel={t.submit}
          pending={form.isPending}
        />
      </form>
    </>
  );
}
