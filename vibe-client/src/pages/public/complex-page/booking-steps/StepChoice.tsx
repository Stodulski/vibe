import { cn } from '@/shared/lib/utils';

interface Choice {
  value: string;
  label: string;
}

interface StepChoiceProps {
  question: string;
  choices: Choice[];
  selected: string | null;
  onChoose: (value: string) => void;
}

/**
 * One question of the guided flow: a heading and its answers.
 *
 * The answers are large targets in a single column on a phone, because at
 * this point in the flow there is nothing else on screen competing for the
 * space, and a question worth asking is worth answering without aiming.
 *
 * Choosing advances on the tap — no confirm button underneath. That is safe
 * here, and not in the grid of hours: these are two or three options with
 * names, presented alone; the grid is dozens of small adjacent targets, which
 * is where a tap that navigates becomes a tap that misfires.
 */
/**
 * How many columns a given number of answers wants, from `sm` up.
 *
 * This was a fixed `sm:grid-cols-3`, which is right for the three durations and
 * wrong for anything else: a venue offering four sports got three across and a
 * fourth stranded alone on its own row, which reads as a mistake rather than as
 * a choice. Four splits evenly into two rows of two; one, two or three stay on
 * a single row.
 *
 * Explicit class strings rather than an interpolated `sm:grid-cols-${n}`:
 * Tailwind scans source text, so a class it never sees written is a class it
 * never generates, and the grid would silently fall back to one column.
 */
const COLUMNS_FOR: Record<number, string> = {
  1: 'sm:grid-cols-1',
  2: 'sm:grid-cols-2',
  3: 'sm:grid-cols-3',
  4: 'sm:grid-cols-2',
};

export function StepChoice({ question, choices, selected, onChoose }: StepChoiceProps) {
  const columns = COLUMNS_FOR[choices.length] ?? 'sm:grid-cols-3';

  return (
    <section>
      <h2 className="text-base font-semibold text-text-primary sm:text-lg">{question}</h2>
      <div className={cn('mt-3 grid gap-2', columns)}>
        {choices.map((choice) => (
          <button
            key={choice.value}
            onClick={() => {
              onChoose(choice.value);
            }}
            aria-pressed={choice.value === selected}
            className={cn(
              // h-12 rather than h-14: still comfortably past the 48px floor,
              // and a four-answer question in two rows is now a third shorter
              // than it was in one row plus an orphan.
              'flex h-12 cursor-pointer items-center justify-center rounded-xl border px-4 text-base font-semibold transition-colors duration-200 press-scale',
              choice.value === selected
                ? 'border-primary-500 bg-primary-500/10 text-primary-400'
                : 'border-border-subtle bg-bg-subtle text-text-primary hover:border-border-default hover:bg-bg-overlay',
            )}
          >
            {choice.label}
          </button>
        ))}
      </div>
    </section>
  );
}
