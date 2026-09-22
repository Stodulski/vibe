import { cn, formatPrice } from '@/shared/lib/utils';

/** One "label: amount" line — the close dialog's expected-cash hint and its closed result's own figures. */
export function CashHintRow({ label, value, emphasize }: { label: string; value: number; emphasize?: boolean }) {
  return (
    <div
      className={cn('flex items-center justify-between text-sm', emphasize && 'bg-bg-base rounded-xl px-3.5 py-2.5')}
    >
      <span className="text-text-tertiary">{label}</span>
      <span className="font-semibold">{formatPrice(value)}</span>
    </div>
  );
}
