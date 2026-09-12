import { ES_AR } from '@/shared/i18n/es_AR';
import { formatDateLong } from '@/shared/lib/utils';
import { useMPFees } from '../../hooks/useMPFees';
import { matchMPFeesGroup } from '../../utils/matchMPFeesGroup';

const t = ES_AR;

const rateFormatter = new Intl.NumberFormat('es-AR', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

function formatRate(rate: number): string {
  return `${rateFormatter.format(rate)}%`;
}

function SourceLink({ href }: { href: string }) {
  return (
    <a href={href} target="_blank" rel="noopener noreferrer" className="text-primary-400 underline">
      {t.mp.feesSourceLink}
    </a>
  );
}

export function MPFeesSection({ province }: { province: string }) {
  const { data, isLoading, isError } = useMPFees();

  // No spinner: this sits under a card that already finished loading, and a
  // second spinner appearing below it reads as the page still not being ready.
  if (isLoading) {
    return <div className="bg-bg-elevated mt-4 h-32 animate-pulse rounded-lg" aria-hidden="true" />;
  }

  const grupo = data ? matchMPFeesGroup(province, data.grupos) : undefined;

  if (isError || !data) {
    return <p className="border-border-subtle text-text-tertiary mt-4 border-t pt-4 text-xs">{t.mp.feesLoadError}</p>;
  }

  if (!grupo) {
    return (
      <p className="border-border-subtle text-text-tertiary mt-4 border-t pt-4 text-xs">
        {t.mp.feesNotFound} {province}. <SourceLink href={data.fuente} />
      </p>
    );
  }

  return (
    <div className="border-border-subtle mt-4 space-y-3 border-t pt-4">
      <p className="text-text-tertiary text-xs font-semibold tracking-wider uppercase">
        {t.mp.feesTitleIn} {province}
      </p>

      <table className="w-full text-sm" aria-label={t.mp.feesTitle}>
        <thead>
          <tr className="border-border-subtle border-b text-left">
            <th scope="col" className="text-text-tertiary pr-4 pb-2 text-xs font-medium">
              {t.mp.feesPlazo}
            </th>
            <th scope="col" className="text-text-tertiary pb-2 text-right text-xs font-medium">
              {t.mp.feesRate}
            </th>
          </tr>
        </thead>
        <tbody>
          {data.plazos.map((plazo, index) => (
            <tr key={plazo} className="border-border-subtle/50 border-b last:border-0">
              <td className="text-text-secondary py-2 pr-4">{plazo}</td>
              <td className="text-text-primary py-2 text-right font-medium tabular-nums">
                {formatRate(grupo.tasas[index] ?? 0)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <p className="text-micro text-text-tertiary">{t.mp.feesDisclaimer}</p>
      <p className="text-micro text-text-tertiary">
        {t.mp.feesValidFrom} {formatDateLong(data.vigente_desde)} · <SourceLink href={data.fuente} />
      </p>
    </div>
  );
}
