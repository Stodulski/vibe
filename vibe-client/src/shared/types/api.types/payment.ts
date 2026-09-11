// ─── Payment ───

export interface Payment {
  id: string;
  booking_id: string;
  complex_id: string;
  amount: number;
  service_fee: number;
  method: string;
  status: string;
  mp_payment_id?: string;
  mp_preference_id?: string;
  refund_amount: number;
  created_at: string;
  updated_at: string;
}
