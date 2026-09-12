import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { confirmPaymentSchema, type ConfirmPaymentDto } from '../../schemas/booking.schema';
import type { Booking } from '@/shared/types/api.types';

/**
 * Mounted fresh per booking (`ConfirmPaymentModal` keys its body on
 * `booking?.id`), so `defaultValues` below is already this booking's own
 * remaining amount from the first render — no "reset when booking changes"
 * effect needed, the same key-instead-of-reset pattern as `CourtForm`/
 * `BlockSlotModal`.
 */
export function useConfirmPaymentForm({ booking }: { booking: Booking | null }) {
  const isDepositPaid = booking?.collection_status === 'deposit_paid';
  const remaining = isDepositPaid ? booking.price - booking.deposit_amount : (booking?.price ?? 0);

  const [paymentType, setPaymentType] = useState<'full' | 'deposit'>('full');

  const {
    handleSubmit,
    setValue,
    register,
    formState: { errors },
  } = useForm<ConfirmPaymentDto>({
    resolver: zodResolver(confirmPaymentSchema),
    defaultValues: { method: 'cash', amount: remaining },
  });

  const handlePaymentTypeChange = (type: 'full' | 'deposit') => {
    setPaymentType(type);
    setValue('amount', type === 'full' ? remaining : 0);
  };

  return {
    handleSubmit,
    setValue,
    register,
    errors,
    isDepositPaid,
    remaining,
    paymentType,
    handlePaymentTypeChange,
  };
}
