import { useState } from 'react';
import { Loader2, StickyNote } from 'lucide-react';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { Textarea } from '@/shared/components/ui/textarea';
import { Button } from '@/shared/components/ui/button';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { COUNTER_PAYMENT_METHODS, type CounterPaymentMethod } from '@/shared/lib/paymentMethods';

const t = ES_AR;

export interface SellCartFooterProps {
  total: number;
  method: CounterPaymentMethod;
  onMethodChange: (method: CounterPaymentMethod) => void;
  note: string;
  onNoteChange: (note: string) => void;
  onCharge: () => void;
  isCharging: boolean;
  chargeDisabled: boolean;
}

/** The payment method, labelled for screen readers; the row has no room for a visible label. */
function CartMethodSelect({
  method,
  onMethodChange,
}: {
  method: CounterPaymentMethod;
  onMethodChange: (method: CounterPaymentMethod) => void;
}) {
  return (
    <Select
      value={method}
      onValueChange={(v) => {
        onMethodChange(v as CounterPaymentMethod);
      }}
    >
      <SelectTrigger aria-label={t.cash.movementMethod} className="h-11 min-w-0 flex-1">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {COUNTER_PAYMENT_METHODS.map((m) => (
          <SelectItem key={m} value={m}>
            {t.bookings.paymentMethods[m]}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/**
 * Method and note on one compact row: the method as a full-width select, the
 * note as an icon toggle beside it, since most sales carry none. The open note
 * field shows below the row.
 */
function CartMethodAndNote({
  method,
  onMethodChange,
  note,
  onNoteChange,
}: {
  method: CounterPaymentMethod;
  onMethodChange: (method: CounterPaymentMethod) => void;
  note: string;
  onNoteChange: (note: string) => void;
}) {
  const [noteOpen, setNoteOpen] = useState(note !== '');

  return (
    <>
      <div className="flex items-center gap-2">
        <CartMethodSelect method={method} onMethodChange={onMethodChange} />
        <Button
          type="button"
          variant="outline"
          size="icon-lg"
          className="size-11 shrink-0"
          aria-label={t.cash.cartAddNote}
          aria-pressed={noteOpen}
          onClick={() => {
            setNoteOpen((open) => !open);
          }}
        >
          <StickyNote className="size-4" aria-hidden="true" />
        </Button>
      </div>
      {noteOpen && (
        <Textarea
          id="sell-cart-note"
          aria-label={`${t.cash.cartNote} (${t.common.optional})`}
          placeholder={`${t.cash.cartNote} (${t.common.optional})`}
          value={note}
          onChange={(e) => {
            onNoteChange(e.target.value);
          }}
          maxLength={500}
          className="min-h-[40px] resize-none"
        />
      )}
    </>
  );
}

/**
 * Total, method, note and "Cobrar $X", kept short so the lines above get the
 * room. The charge button is the one green element of the cart.
 */
export function SellCartFooter({
  total,
  method,
  onMethodChange,
  note,
  onNoteChange,
  onCharge,
  isCharging,
  chargeDisabled,
}: SellCartFooterProps) {
  return (
    <div className="border-border-subtle mt-2 shrink-0 space-y-3 border-t pt-4">
      <div className="flex items-baseline justify-between gap-3">
        <span className="text-text-secondary text-sm">{t.cash.cartTotal}</span>
        <span
          className="font-display text-text-primary text-2xl font-bold tracking-tight tabular-nums"
          data-testid="sell-cart-total"
        >
          {formatPrice(total)}
        </span>
      </div>

      <CartMethodAndNote method={method} onMethodChange={onMethodChange} note={note} onNoteChange={onNoteChange} />

      <Button
        type="button"
        className="h-12 w-full text-base"
        disabled={isCharging || chargeDisabled}
        onClick={onCharge}
      >
        {isCharging && <Loader2 className="size-4 animate-spin" aria-hidden="true" />}
        {`${t.cash.cartChargeActionPrefix} ${formatPrice(total)}`}
      </Button>
    </div>
  );
}
