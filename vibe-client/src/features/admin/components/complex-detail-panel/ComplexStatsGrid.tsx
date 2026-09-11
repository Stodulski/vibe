import { Users, CalendarDays, DollarSign, Trophy } from 'lucide-react';
import type { AdminComplexDetailResponse } from '@/shared/types/api.types';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { StatTile, type StatTileTone } from '@/shared/components/common/StatTile';

const t = ES_AR;

interface ComplexStatsGridProps {
  data: AdminComplexDetailResponse;
}

export function ComplexStatsGrid({ data }: ComplexStatsGridProps) {
  const stats = [
    {
      label: t.admin.complexes.courts,
      value: data.courts_count,
      icon: Trophy,
      iconTone: 'orange' as StatTileTone,
    },
    {
      label: t.admin.complexes.totalClients,
      value: data.clients_count.toLocaleString('es-AR'),
      icon: Users,
      iconTone: 'blue' as StatTileTone,
    },
    {
      label: t.admin.complexes.totalBookings,
      value: data.bookings_count.toLocaleString('es-AR'),
      icon: CalendarDays,
      iconTone: 'purple' as StatTileTone,
    },
    {
      label: t.admin.complexes.totalRevenue,
      value: formatPrice(data.total_revenue),
      icon: DollarSign,
      iconTone: 'emerald' as StatTileTone,
    },
  ];

  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      {stats.map((stat) => (
        <StatTile key={stat.label} label={stat.label} value={stat.value} icon={stat.icon} iconTone={stat.iconTone} />
      ))}
    </div>
  );
}
