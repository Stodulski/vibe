import { BOOKING_MESSAGES } from '../../messages';
import { useBookingConfig } from '../../config';

const t = BOOKING_MESSAGES;

/** The line under the pay button: who manages the data, and the link to the policy. One for every checkout. */
export function PrivacyNotice({ text }: { text: string }) {
  const { privacyUrl } = useBookingConfig();
  return (
    <p className="text-text-tertiary text-xs">
      {text}{' '}
      <a
        href={privacyUrl}
        target="_blank"
        rel="noopener noreferrer"
        className="text-primary-400 underline underline-offset-2"
      >
        {t.publicBooking.privacyNoticeLink}
      </a>
    </p>
  );
}
