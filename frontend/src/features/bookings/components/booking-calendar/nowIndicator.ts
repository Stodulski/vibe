/**
 * Minutes-since-midnight for "now" in the app's timezone, when `date`
 * (YYYY-MM-DD) is today there. Returns `null` for any other day so the
 * timeline only ever draws the now-line on the current day.
 */
export function getNowMinutesInBuenosAires(date: string): number | null {
  const formatter = new Intl.DateTimeFormat('en-CA', {
    timeZone: 'America/Argentina/Buenos_Aires',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  });
  const parts = formatter.formatToParts(new Date());
  const get = (type: string): string => parts.find((p) => p.type === type)?.value ?? '';

  const todayStr = `${get('year')}-${get('month')}-${get('day')}`;
  if (todayStr !== date) return null;

  const hours = Number(get('hour'));
  const minutes = Number(get('minute'));
  if (!Number.isFinite(hours) || !Number.isFinite(minutes)) return null;

  return (hours % 24) * 60 + minutes;
}
