import { useState } from 'react';
import { Loader2 } from 'lucide-react';
import { AlertDialog, AlertDialogContent } from '@/shared/components/common/AppAlertDialog';
import {
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from '@/shared/components/ui/alert-dialog';
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
    <AlertDialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t.cash.saleVoidConfirmTitle}</AlertDialogTitle>
          <AlertDialogDescription>{t.cash.saleVoidConfirmDescription}</AlertDialogDescription>
        </AlertDialogHeader>

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

        <AlertDialogFooter>
          <AlertDialogCancel disabled={voidSale.isPending}>{t.common.cancel}</AlertDialogCancel>
          <AlertDialogAction
            onClick={(e) => {
              e.preventDefault();
              if (!sale) return;
              voidSale.mutate({ saleId: sale.id, note: blankToUndefined(note) }, { onSuccess: onClose });
            }}
            disabled={voidSale.isPending}
            className="bg-destructive hover:bg-destructive/90 text-white"
          >
            {voidSale.isPending && <Loader2 className="size-4 animate-spin" />}
            {t.cash.saleVoidAction}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
