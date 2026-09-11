import {
  Circle,
  CircleCheck,
  CircleDot,
  CircleEllipsis,
  CircleMinus,
  CircleX,
  Clock,
  RotateCcw,
  UserCheck,
  UserX,
  type LucideIcon,
} from 'lucide-react';
import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { BookingStatus, PaymentDisplayStatus } from '@/shared/types/api.types';

// Colour alone carries the state — no pill behind it. The filled shape read as
// something you could press, and on a card already full of chips it competed
// with the booking's own information for attention.
//
// Amber means "money still owed" (a deposit, a refund on its way, a no-show to
// settle), green means "done", red means "cancelled". Nothing else is used, so
// the same colour means the same thing on every screen.
const STATUS_STYLES: Record<BookingStatus, string> = {
  pending: 'text-warning-text',
  confirmed: 'text-success-text',
  cancelled: 'text-error-text',
  completed: 'text-success-text',
  no_show: 'text-warning-text',
};

// The same three semantics as `STATUS_STYLES`, as a solid filled shape for
// surfaces that read a status from the block itself (the court grid) rather
// than from an icon beside it. An alpha fill was tried first and rejected: it
// let the gridlines show through and shifted with whatever sat behind it.
//
// The fill is the `-border` tone, not the `-bg` one — `-bg` is a near-black
// tint built to sit *behind* text, and on a dark grid it was indistinguishable
// from an empty cell. These read as the colour they mean and still clear
// 4.5:1 against `text-text-primary` (measured: 7.1 for the green).
// eslint-disable-next-line react-refresh/only-export-components -- a plain style map, not a component; kept here so the status semantics live in one reviewed place.
export const STATUS_BAR_STYLES: Record<BookingStatus, { bg: string; border: string }> = {
  pending: { bg: 'bg-warning-border', border: 'border-warning-icon' },
  confirmed: { bg: 'bg-success-border', border: 'border-success-icon' },
  cancelled: { bg: 'bg-error-border', border: 'border-error-icon' },
  completed: { bg: 'bg-success-border', border: 'border-success-icon' },
  no_show: { bg: 'bg-warning-border', border: 'border-warning-icon' },
};

// Keyed by PaymentDisplayStatus, the one-word projection of the booking's
// collection_status/refund_status pair (see api.types/booking.ts). A badge has
// room for one word; the pair is there for anywhere that has room for both.
const PAYMENT_STYLES: Record<PaymentDisplayStatus, string> = {
  unpaid: 'text-warning-text',
  deposit_paid: 'text-warning-text',
  fully_paid: 'text-success-text',
  refunded: 'text-text-secondary',
  refund_pending: 'text-warning-text',
  partial_refund: 'text-warning-text',
};

// Icon-only variants for dense lists, where the state is a glance rather than a
// read. The colour system above still applies, but colour alone can't carry it:
// `pending` and `no_show` are both amber, so each state gets its own glyph, and
// every icon exposes its full label to the accessibility tree and as a tooltip.
const STATUS_ICONS: Record<BookingStatus, LucideIcon> = {
  pending: Clock,
  confirmed: CircleCheck,
  cancelled: CircleX,
  completed: UserCheck,
  no_show: UserX,
};

const PAYMENT_ICONS: Record<PaymentDisplayStatus, LucideIcon> = {
  unpaid: Circle,
  deposit_paid: CircleDot,
  fully_paid: CircleCheck,
  refunded: RotateCcw,
  refund_pending: CircleEllipsis,
  partial_refund: CircleMinus,
};

function StatusIcon({ Icon, label, className }: { Icon: LucideIcon; label: string; className?: string }) {
  return (
    <span role="img" aria-label={label} title={label} className={cn('shrink-0', className)}>
      <Icon className="size-4" aria-hidden="true" />
    </span>
  );
}

export function BookingStatusIcon({ status }: { status: BookingStatus }) {
  return (
    <StatusIcon Icon={STATUS_ICONS[status]} label={ES_AR.bookings.statuses[status]} className={STATUS_STYLES[status]} />
  );
}

export function PaymentStatusIcon({ status }: { status: PaymentDisplayStatus }) {
  return (
    <StatusIcon
      Icon={PAYMENT_ICONS[status]}
      label={ES_AR.bookings.paymentStatuses[status]}
      className={PAYMENT_STYLES[status]}
    />
  );
}

export function BookingStatusBadge({ status }: { status: BookingStatus }) {
  return (
    <span className={cn('shrink-0 whitespace-nowrap text-sm font-semibold', STATUS_STYLES[status])}>
      {ES_AR.bookings.statuses[status]}
    </span>
  );
}

export function PaymentStatusBadge({ status }: { status: PaymentDisplayStatus }) {
  return (
    <span className={cn('shrink-0 whitespace-nowrap text-sm font-semibold', PAYMENT_STYLES[status])}>
      {ES_AR.bookings.paymentStatuses[status]}
    </span>
  );
}

// Same colours at the weight a labelled detail row wants, where the value sits
// under its own label rather than beside other statuses.
export function BookingStatusText({ status, className }: { status: BookingStatus; className?: string }) {
  return <span className={cn('font-medium', STATUS_STYLES[status], className)}>{ES_AR.bookings.statuses[status]}</span>;
}

export function PaymentStatusText({ status, className }: { status: PaymentDisplayStatus; className?: string }) {
  return (
    <span className={cn('font-medium', PAYMENT_STYLES[status], className)}>
      {ES_AR.bookings.paymentStatuses[status]}
    </span>
  );
}
