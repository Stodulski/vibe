import { Download, Loader2 } from 'lucide-react';
import { MONTH_NAMES } from './constants';
import { ES_AR } from '@/shared/i18n/es_AR';
import { Button } from '@/shared/components/ui/button';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';

const t = ES_AR;
interface MonthOption {
  value: number;
  label: string;
  disabled: boolean;
}

interface ExportButtonProps {
  exporting: boolean;
  disabled: boolean;
  /** Label shown while `exporting`. Defaults to `t.reports.downloading` — see the caller. */
  statusLabel?: string;
  onExport: () => void;
}

// The app's own primary button, not a hand-rolled one. This was a bare
// <button> carrying its own green, its own radius and its own disabled
// styling — a second definition of the thing `Button` already defines, which
// drifts the moment either is restyled.
function ExportButton({ exporting, disabled, statusLabel, onExport }: ExportButtonProps) {
  return (
    // `lg` because a SelectTrigger is 40px and the default Button is 36: the
    // bar aligns its controls at the bottom, so the shorter one starts four
    // pixels lower and reads as dropped rather than as smaller.
    <Button
      size="lg"
      // Full width on a phone. The two pickers fit side by side at 320px and
      // the button wraps below them, where its natural width leaves it looking
      // like a stray third picker rather than the action the row builds to.
      className="w-full sm:w-auto"
      onClick={onExport}
      disabled={disabled}
      aria-busy={exporting}
    >
      {exporting ? (
        <Loader2 className="size-4 animate-spin" aria-hidden="true" />
      ) : (
        <Download className="size-4" aria-hidden="true" />
      )}
      {exporting ? (statusLabel ?? t.reports.downloading) : t.reports.downloadExcel}
    </Button>
  );
}

function MonthPicker({
  month,
  availableMonths,
  onMonthChange,
}: {
  month: number;
  availableMonths: MonthOption[];
  onMonthChange: (month: number) => void;
}) {
  return (
    <div className="flex-[2] space-y-1 sm:flex-none">
      <Label htmlFor="report-month">{t.reports.month}</Label>
      <Select
        value={String(month)}
        onValueChange={(v) => {
          onMonthChange(Number(v));
        }}
      >
        <SelectTrigger id="report-month" className="w-full sm:w-36">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {availableMonths.map((m) => (
            <SelectItem key={m.value} value={String(m.value)} disabled={m.disabled}>
              {m.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

function YearPicker({
  year,
  minYear,
  maxYear,
  onYearChange,
}: {
  year: number;
  minYear: number;
  maxYear: number;
  onYearChange: (year: number) => void;
}) {
  return (
    <div className="flex-1 space-y-1 sm:flex-none">
      <Label htmlFor="report-year">{t.reports.year}</Label>
      <Select
        value={String(year)}
        onValueChange={(v) => {
          onYearChange(Number(v));
        }}
      >
        <SelectTrigger id="report-year" className="w-full sm:w-28">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {Array.from({ length: maxYear - minYear + 1 }, (_, i) => minYear + i).map((y) => (
            <SelectItem key={y} value={String(y)}>
              {y}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

interface MonthYearExportBarProps {
  month: number;
  year: number;
  minYear: number;
  maxYear: number;
  availableMonths: MonthOption[];
  isLoading: boolean;
  exporting: boolean;
  exportStatusLabel?: string;
  onMonthChange: (month: number) => void;
  onYearChange: (year: number) => void;
  onExport: () => void;
}

export function MonthYearExportBar({
  month,
  year,
  minYear,
  maxYear,
  availableMonths,
  isLoading,
  exporting,
  exportStatusLabel,
  onMonthChange,
  onYearChange,
  onExport,
}: MonthYearExportBarProps) {
  return (
    <div className="mb-4 flex flex-wrap items-end gap-3 sm:mb-6">
      {/* The app's Select, not the browser's. These were native <select>
          elements with hand-written Tailwind, so they rendered as the
          operating system's dropdown — a different shape, a different
          typeface and a different open behaviour from every other choice in
          the app, on the one screen an owner reads their money on. */}
      {/* The two pickers share the row on a phone and keep their own widths
          from `sm` up, where a month picker as wide as the card reads as a
          field waiting to be typed into rather than a choice.

          Two parts to one: the month holds "Septiembre" and the year holds
          four digits, so an even split gives the year room it has no use for
          and takes it from the one that needs it. */}
      <MonthPicker month={month} availableMonths={availableMonths} onMonthChange={onMonthChange} />

      <YearPicker year={year} minYear={minYear} maxYear={maxYear} onYearChange={onYearChange} />

      <ExportButton
        exporting={exporting}
        disabled={exporting || isLoading}
        {...(exportStatusLabel !== undefined ? { statusLabel: exportStatusLabel } : {})}
        onExport={onExport}
      />
    </div>
  );
}

export { MONTH_NAMES };
