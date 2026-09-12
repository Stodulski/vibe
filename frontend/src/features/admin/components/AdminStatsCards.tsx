import { memo } from 'react';
import type { PlatformStats } from '@/shared/types/api.types';
import { formatPrice, cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { StatTile } from '@/shared/components/common/StatTile';

const t = ES_AR;

interface AdminStatsCardsProps {
  stats: PlatformStats;
}

export const AdminStatsCards = memo(function AdminStatsCards({ stats }: AdminStatsCardsProps) {
  const cards = [
    { label: t.admin.stats.totalUsers, value: String(stats.total_users) },
    { label: t.admin.stats.activeUsers, value: String(stats.active_users) },
    { label: t.admin.stats.newUsersMonth, value: String(stats.new_users_month) },
    { label: t.admin.stats.totalComplexes, value: String(stats.total_complexes) },
    { label: t.admin.stats.newComplexesMonth, value: String(stats.new_complexes_month) },
    { label: t.admin.stats.totalCourts, value: String(stats.total_courts) },
    { label: t.admin.stats.totalBookings, value: stats.total_bookings.toLocaleString('es-AR') },
    { label: t.admin.stats.totalRevenue, value: formatPrice(stats.total_revenue), isMono: true },
  ];

  return (
    <div
      className="stagger-children grid grid-cols-2 gap-2 sm:gap-3 lg:grid-cols-4"
      role="region"
      aria-label={t.admin.stats.title}
    >
      {cards.map((card) => (
        <StatTile
          key={card.label}
          label={card.label}
          value={card.value}
          valueClassName={cn(card.isMono && 'score-text')}
        />
      ))}
    </div>
  );
});
