import { Button } from '@/shared/components/ui/button';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CartLine } from '../../lib/cart';

const t = ES_AR;

interface SellMobileCartBarProps {
  lines: CartLine[];
  total: number;
  onOpenCart: () => void;
}

/** Sticky bottom bar — the cart's mobile collapsed form (`odd/tasks/pos-cashbox.md` T5b). Hidden with an empty cart: nothing to view or charge yet. */
export function SellMobileCartBar({ lines, total, onOpenCart }: SellMobileCartBarProps) {
  if (lines.length === 0) return null;
  const itemCount = lines.reduce((sum, line) => sum + line.quantity, 0);

  return (
    <div className="border-border-subtle bg-bg-elevated fixed inset-x-0 bottom-0 z-30 border-t px-4 py-3 lg:hidden">
      <Button
        type="button"
        className="flex h-12 w-full items-center justify-between px-4 text-base"
        onClick={onOpenCart}
      >
        <span>{`${t.cash.cartViewAction} (${String(itemCount)})`}</span>
        <span className="score-text font-bold">{formatPrice(total)}</span>
      </Button>
    </div>
  );
}
