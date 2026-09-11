import { useEffect, useState } from 'react';
import { format } from 'date-fns/format';
import { formatDateFull } from '@/shared/lib/utils';

function formatNow(): string {
  const now = new Date();
  return `${formatDateFull(now)} · ${format(now, 'HH:mm')}`;
}

/** Live-updating "day, date · time" string — the dashboard masthead subtitle. */
export function useLiveClock(): string {
  const [text, setText] = useState(formatNow);

  useEffect(() => {
    const id = setInterval(() => {
      setText(formatNow());
    }, 30_000);
    return () => {
      clearInterval(id);
    };
  }, []);

  return text;
}
