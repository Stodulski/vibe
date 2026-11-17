import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
  Button,
} from '@vibe/ui';

export interface AgendaDetailFact {
  term: string;
  /** One line, or a list of them shown one under the other. */
  value: string | string[];
}

/** What the detail of one agenda row says: its title, its day and time, its facts and its sign-up. */
export interface AgendaDetail {
  title: string;
  when: string;
  facts: AgendaDetailFact[];
  signUp: { href: string; label: string };
}

interface AgendaDetailDialogProps {
  /** The detail that is open, or `null` while it is closed. */
  detail: AgendaDetail | null;
  closeLabel: string;
  onClose: () => void;
}

function Fact({ term, value }: AgendaDetailFact) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-text-tertiary text-xs font-semibold tracking-wide uppercase">{term}</dt>
      <dd className="text-text-primary text-sm">
        {typeof value === 'string' ? (
          value
        ) : (
          <ul className="flex flex-col gap-0.5">
            {value.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        )}
      </dd>
    </div>
  );
}

/**
 * The detail of an agenda row, in a dialog. Closing it, with the button or with
 * Escape, returns the focus to the row.
 */
export function AgendaDetailDialog({ detail, closeLabel, onClose }: AgendaDetailDialogProps) {
  return (
    <AlertDialog
      open={detail !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <AlertDialogContent className="bg-bg-subtle border-border-subtle flex flex-col gap-5 rounded-2xl border p-6 shadow-lg sm:max-w-md">
        {detail && (
          <>
            <div className="flex flex-col gap-1">
              <AlertDialogTitle className="text-text-primary text-xl leading-tight font-bold">{detail.title}</AlertDialogTitle>
              <AlertDialogDescription className="text-text-secondary text-sm">{detail.when}</AlertDialogDescription>
            </div>

            <dl className="flex flex-col gap-4">
              {detail.facts.map((fact) => (
                <Fact key={fact.term} term={fact.term} value={fact.value} />
              ))}
            </dl>

            <div className="flex flex-col gap-2 pt-2 sm:flex-row">
              <Button asChild size="lg" className="min-h-12 w-full sm:w-auto">
                <a href={detail.signUp.href}>{detail.signUp.label}</a>
              </Button>
              <Button variant="outline" size="lg" className="min-h-12 w-full sm:w-auto" onClick={onClose}>
                {closeLabel}
              </Button>
            </div>
          </>
        )}
      </AlertDialogContent>
    </AlertDialog>
  );
}
