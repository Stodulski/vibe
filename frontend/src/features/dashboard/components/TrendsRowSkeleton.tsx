import { ClientInsightsCardSkeleton } from './ClientInsightsCardSkeleton';
import { RevenueChartSkeleton } from './RevenueChartSkeleton';
import { TRENDS_MAIN_CELL_CLASS, TRENDS_ROW_CLASS, TRENDS_SIDE_CELL_CLASS } from './dashboardLayout';

/**
 * Loading shape of the revenue + client insights row. At `xl` the row keeps
 * the measured loaded height (~610px); below it the cards stack and keep
 * their own natural height.
 */
export function TrendsRowSkeleton() {
  return (
    <div className={TRENDS_ROW_CLASS}>
      <div className={TRENDS_MAIN_CELL_CLASS}>
        <RevenueChartSkeleton className="xl:min-h-[600px]" />
      </div>
      <div className={TRENDS_SIDE_CELL_CLASS}>
        <ClientInsightsCardSkeleton />
      </div>
    </div>
  );
}
