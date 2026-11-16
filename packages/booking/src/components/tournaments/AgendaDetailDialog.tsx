import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
  Button,
  formatPrice,
} from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { publicTournamentPath } from '../../lib/publicPaths';
import { categoryWhen, formatDetailLabel, type AgendaRow } from './agendaModel';

const t = BOOKING_MESSAGES.tournaments;

interface AgendaDetailDialogProps {
  /** The row whose detail is open, or `null` while it is closed. */
  row: AgendaRow | null;
  slug: string;
  onClose: () => void;
}

function Fact({ term, children }: { term: string; children: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-text-tertiary text-xs font-semibold tracking-wide uppercase">{term}</dt>
      <dd className="text-text-primary text-sm">{children}</dd>
    </div>
  );
}

function DetailBody({ row, slug, onClose }: { row: AgendaRow; slug: string; onClose: () => void }) {
  const { category } = row;
  return (
    <>
      <div className="flex flex-col gap-1">
        <AlertDialogTitle className="text-text-primary text-xl leading-tight font-bold">{row.title}</AlertDialogTitle>
        <AlertDialogDescription className="text-text-secondary text-sm">{categoryWhen(row)}</AlertDialogDescription>
      </div>

      <dl className="flex flex-col gap-4">
        <Fact term={t.inscriptionLabel}>{`${formatPrice(category.price)} ${t.perPair}`}</Fact>
        {category.prizes && <Fact term={t.prizesLabel}>{category.prizes}</Fact>}
        <Fact term={t.formatLabel}>{formatDetailLabel(category)}</Fact>
      </dl>

      <div className="flex flex-col gap-2 pt-2 sm:flex-row">
        <Button asChild size="lg" className="min-h-12 w-full sm:w-auto">
          <a href={publicTournamentPath(slug, row.tournamentId, category.id)}>{t.signUpCategory}</a>
        </Button>
        <Button variant="outline" size="lg" className="min-h-12 w-full sm:w-auto" onClick={onClose}>
          {t.closeDetail}
        </Button>
      </div>
    </>
  );
}

/**
 * The detail of one agenda row: the category's title, its day and time, what a
 * pair pays, its prizes when it has them, and its format, with the sign-up.
 * Closing it, with the button or with Escape, returns the focus to the row.
 */
export function AgendaDetailDialog({ row, slug, onClose }: AgendaDetailDialogProps) {
  return (
    <AlertDialog
      open={row !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <AlertDialogContent className="bg-bg-subtle border-border-subtle flex flex-col gap-5 rounded-2xl border p-6 shadow-lg sm:max-w-md">
        {row && <DetailBody row={row} slug={slug} onClose={onClose} />}
      </AlertDialogContent>
    </AlertDialog>
  );
}
