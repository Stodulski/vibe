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
    return <div className="mt-4 h-32 animate-pulse rounded-lg bg-bg-elevated" aria-hidden="true" />;
  }

  const grupo = data ? matchMPFeesGroup(province, data.grupos) : undefined;

  if (isError || !data) {
    return <p className="mt-4 border-t border-border-subtle pt-4 text-xs text-text-tertiary">{t.mp.feesLoadError}</p>;
  }

  if (!grupo) {
    return (
      <p className="mt-4 border-t border-border-subtle pt-4 text-xs text-text-tertiary">
        {t.mp.feesNotFound} {province}. <SourceLink href={data.fuente} />
      </p>
    );
  }

  return (
    <div className="mt-4 space-y-3 border-t border-border-subtle pt-4">
      <p className="text-xs font-semibold uppercase tracking-wider text-text-tertiary">
        {t.mp.feesTitleIn} {province}
      </p>

      <table className="w-full text-sm" aria-label={t.mp.feesTitle}>
        <thead>
          <tr className="border-b border-border-subtle text-left">
            <th scope="col" className="pb-2 pr-4 text-xs font-medium text-text-tertiary">
              {t.mp.feesPlazo}
            </th>
            <th scope="col" className="pb-2 text-right text-xs font-medium text-text-tertiary">
              {t.mp.feesRate}
            </th>
          </tr>
        </thead>
        <tbody>
          {data.plazos.map((plazo, index) => (
            <tr key={plazo} className="border-b border-border-subtle/50 last:border-0">
              <td className="py-2 pr-4 text-text-secondary">{plazo}</td>
              <td className="py-2 text-right font-medium text-text-primary tabular-nums">
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
