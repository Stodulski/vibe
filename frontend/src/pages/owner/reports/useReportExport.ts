import { useState } from 'react';
import { HTTPError } from 'ky';
import { dashboardApi } from '@/features/dashboard';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';

const t = ES_AR;

export function useReportExport(complexId: string, month: number, year: number) {
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);

  const handleExport = async () => {
    setExporting(true);
    setExportError(null);
    try {
      const blob = await dashboardApi.exportPaymentsExcel(complexId, month, year);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `pagos_${String(month)}_${String(year)}.xlsx`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (error) {
      // `.blob()` still throws a `ky` HTTPError on a non-2xx response, and
      // `error.data` is populated from the JSON error body regardless of
      // which body method the caller used to read a successful response —
      // it must be read here, not discarded, or a real server code (too
      // large, timed out) is indistinguishable from a network hiccup.
      setExportError(
        error instanceof HTTPError
          ? getHttpErrorMessage(error, t.dashboard.exportGenericError)
          : t.dashboard.exportGenericError,
      );
    } finally {
      setExporting(false);
    }
  };

  return { exporting, exportError, handleExport };
}
