import type { ReactNode } from 'react';
import { Button, cn } from '@vibe/ui';
import { useWidth } from '../court-selector/useRulerBox';
import { useWheelPicker } from '../court-selector/useWheelPicker';
import { tickDepth } from '../court-selector/wheelMath';

export interface WheelChoice<T extends string> {
  value: T;
  label: string;
  /** What the card draws above its label. */
  art: ReactNode;
  /** Lines under the label: a court's facts and price. */
  detail?: ReactNode;
  /** The label in full, shown on hover where the card cuts it. Defaults to the label. */
  title?: string;
}

interface ChoiceWheelProps<T extends string> {
  question: string;
  choices: WheelChoice<T>[];
  /** The answer already given, centered when the wheel opens; else the middle choice. */
  selected: T | null;
  /** The confirm button's words, for the choice at the center. */
  continueLabel: string | ((choice: WheelChoice<T>) => string);
  onChoose: (value: T) => void;
  /** Where the wheel opens with nothing chosen: the middle choice, or the first. */
  startAt?: 'middle' | 'first';
  /** Larger cards, for choices with more to say (a court's facts and price). */
  size?: 'regular' | 'large';
}

/** The gap a card takes on the wheel: larger cards and larger screens get more. */
function cardWidth(size: 'regular' | 'large', width: number): number {
  if (size === 'large') return width >= 640 ? 208 : 156;
  return width >= 640 ? 168 : 128;
}

/**
 * Where a card `raw` pixels from the center (at full spacing) is drawn. The cards
 * shrink toward the edges, so the spacing shrinks with them: each step is as wide as
 * the cards around it, and the gaps stay even instead of opening up at the sides.
 * It is the running sum of the card's scale (1.15 at the center, 0.55 at `half`).
 */
function packedOffset(raw: number, half: number): number {
  const reach = Math.max(1, half);
  const u = Math.min(Math.abs(raw), reach);
  const inside = 1.15 * u - (0.3 * u * u) / reach;
  const outside = Math.max(0, Math.abs(raw) - reach) * 0.55;
  return Math.sign(raw) * (inside + outside);
}

const TRACK = 'relative cursor-grab touch-pan-y overflow-hidden select-none active:cursor-grabbing';

/** How high the outer cards climb: lower than the time ruler's, cards are larger. */
const CARD_ARC = 40;

interface WheelCardProps<T extends string> {
  choice: WheelChoice<T>;
  /** Pixels from the wheel's center to this card's center. */
  offset: number;
  half: number;
  width: number;
  focused: boolean;
  onTap: () => void;
}

/** One choice on the wheel, placed and sized by its offset from the center. */
function WheelCard<T extends string>({ choice, offset, half, width, focused, onTap }: WheelCardProps<T>) {
  const depth = tickDepth(offset, half, width, CARD_ARC);
  const x = packedOffset(offset, half);
  return (
    <button
      type="button"
      aria-pressed={focused}
      onClick={onTap}
      style={{
        width,
        left: `calc(50% + ${(x - width / 2).toFixed(1)}px)`,
        opacity: depth.opacity,
        transform: `translateY(${depth.rise.toFixed(1)}px) scale(${depth.scale.toFixed(3)})`,
      }}
      className={cn(
        'absolute bottom-3 flex origin-bottom cursor-pointer flex-col items-center gap-3 rounded-2xl border px-3 pt-4 pb-3',
        focused ? 'border-primary-500 bg-primary-500/10 text-primary-400' : 'border-border-interactive bg-bg-subtle text-text-primary',
      )}
    >
      {choice.art}
      <span className="line-clamp-2 w-full text-base leading-tight font-semibold" title={choice.title ?? choice.label}>
        {choice.label}
      </span>
      {choice.detail}
    </button>
  );
}

/**
 * A question answered on a wheel, in the language of the time ruler: the choices are
 * cards on a curve, the one at the center large and green. A swipe or drag moves it
 * one card at most; tapping a card brings it to the center. The mouse wheel is left
 * to the page. Tapping the centered card, or the button under it, answers.
 */
export function ChoiceWheel<T extends string>({
  question,
  choices,
  selected,
  continueLabel,
  onChoose,
  startAt = 'middle',
  size = 'regular',
}: ChoiceWheelProps<T>) {
  const [width, , measureRef] = useWidth();
  const card = cardWidth(size, width);
  const half = width / 2;
  const chosen = choices.findIndex((choice) => choice.value === selected);
  const opening = startAt === 'first' ? 0 : Math.floor((choices.length - 1) / 2);
  const wheel = useWheelPicker(chosen >= 0 ? chosen : opening, choices.length, card, { stepped: true });
  const current = choices[Math.min(choices.length - 1, Math.max(0, wheel.center))];
  if (!current) return null;

  return (
    <section aria-label={question} className="flex flex-col gap-4">
      <h2 className="text-text-primary text-xl font-bold">{question}</h2>
      <div
        ref={measureRef}
        {...wheel.handlers}
        role="group"
        aria-label={question}
        className={cn(TRACK, size === 'large' ? 'h-80' : 'h-48')}>
        {choices.map((choice, index) => (
          <WheelCard
            key={choice.value}
            choice={choice}
            offset={(index - wheel.pos) * card}
            half={half}
            width={card - 16}
            focused={index === wheel.center}
            onTap={() => {
              if (index === wheel.center) onChoose(choice.value);
              else wheel.goTo(index);
            }}
          />
        ))}
      </div>
      <Button
        size="lg"
        className="h-auto min-h-12 w-full py-3 leading-tight whitespace-normal"
        onClick={() => {
          onChoose(current.value);
        }}
      >
        {typeof continueLabel === 'string' ? continueLabel : continueLabel(current)}
      </Button>
    </section>
  );
}
