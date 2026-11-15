import { useState } from 'react';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { Label } from '@/shared/components/ui/label';
import { Textarea } from '@/shared/components/ui/textarea';
import { ES_AR } from '@/shared/i18n/es_AR';
import { blankToUndefined } from '@/shared/lib/blankToUndefined';
import { useVoidSale } from '../../hooks/useVoidSale';
import type { Sale } from '@/shared/types/api.types';

const t = ES_AR;

interface VoidSaleDialogProps {
  sale: Sale | null;
  onClose: () => void;
  complexId: string;
  sessionId: string;
}

/** Same "AlertDialog + optional Textarea" shape as `VoidMovementDialog`. */
export function VoidSaleDialog({ sale, onClose, complexId, sessionId }: VoidSaleDialogProps) {
  const [note, setNote] = useState('');
  const voidSale = useVoidSale(complexId, sessionId);

  const [lastSaleId, setLastSaleId] = useState(sale?.id);
  if (sale?.id !== lastSaleId) {
    setLastSaleId(sale?.id);
    if (sale) setNote('');
  }

  const open = !!sale;

  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={() => {
        if (!sale) return;
        voidSale.mutate({ saleId: sale.id, note: blankToUndefined(note) }, { onSuccess: onClose });
      }}
      title={t.cash.saleVoidConfirmTitle}
      description={t.cash.saleVoidConfirmDescription}
      confirmLabel={t.cash.saleVoidAction}
      // "Cancelar" next to "Anular" would read as the same action.
      cancelLabel={t.common.back}
      variant="destructive"
      isLoading={voidSale.isPending}
    >
      <div className="space-y-2">
        <Label htmlFor="void-sale-note">{`${t.cash.cartNote} (${t.common.optional})`}</Label>
        <Textarea
          id="void-sale-note"
          value={note}
          onChange={(e) => {
            setNote(e.target.value);
          }}
          placeholder={t.cash.saleVoidNotePlaceholder}
          maxLength={500}
        />
      </div>
    </ConfirmDialog>
  );
}
