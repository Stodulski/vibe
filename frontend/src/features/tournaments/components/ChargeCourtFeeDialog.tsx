import { useState } from 'react';
import { toast } from 'sonner';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/shared/components/common/AppDialog';
import { FormField } from '@/shared/components/common/FormField';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { COUNTER_PAYMENT_METHODS, type CounterPaymentMethod } from '@/shared/lib/paymentMethods';
import { formatPrice } from '@/shared/lib/utils';
import { useChargeCourtFee, type CourtFeeTarget } from '../hooks/useTournamentChanges';
import { courtFeeErrorMessage } from '../lib/tournamentErrors';

const t = ES_AR.tournaments.courtFee;

interface ChargeCourtFeeDialogProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  tournamentId: string;
  /** The pair (a side of a match) being charged; null while closed. */
  target: CourtFeeTarget | null;
  /** The pair's name and what it owes. */
  pairName: string;
  amount: number;
}

/** Collects a pair's court fee with any counter payment method; the money goes into the open cash session. Mounted only while open. */
export function ChargeCourtFeeDialog({ open, onClose, target, ...rest }: ChargeCourtFeeDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        {open && target && <ChargeBody target={target} onClose={onClose} {...rest} />}
      </DialogContent>
    </Dialog>
  );
}

type BodyProps = Omit<ChargeCourtFeeDialogProps, 'open' | 'target'> & { target: CourtFeeTarget };

function ChargeBody({ onClose, complexId, tournamentId, target, pairName, amount }: BodyProps) {
  const [method, setMethod] = useState<CounterPaymentMethod>('cash');
  const charge = useChargeCourtFee(complexId, tournamentId);

  const submit = () => {
    charge.mutate(
      { ...target, method },
      {
        onSuccess: () => {
          toast.success(t.chargeSuccess);
          onClose();
        },
        onError: (error) => {
          toast.error(t.errors.chargeUnknown, { description: courtFeeErrorMessage(error, 'charge') });
        },
      },
    );
  };

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t.chargeTitle}</DialogTitle>
        <DialogDescription>{t.chargeDescription(pairName, formatPrice(amount))}</DialogDescription>
      </DialogHeader>
      <FormField label={t.method} htmlFor="court-fee-method">
        <Select
          value={method}
          onValueChange={(value) => {
            setMethod(COUNTER_PAYMENT_METHODS.find((m) => m === value) ?? 'cash');
          }}
        >
          <SelectTrigger id="court-fee-method">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {COUNTER_PAYMENT_METHODS.map((value) => (
              <SelectItem key={value} value={value}>
                {ES_AR.bookings.paymentMethods[value]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FormField>
      <SectionFooter
        onCancel={onClose}
        cancelDisabled={charge.isPending}
        submitLabel={t.chargeSubmit(formatPrice(amount))}
        pending={charge.isPending}
        onSubmit={submit}
      />
    </>
  );
}
