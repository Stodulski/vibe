import type { ChartDatum } from './chart-draw';

/**
 * What `ChartBody` is showing, as one value instead of the
 * `isLoading` + `isError` + `onRetry` + `chartData` quadruple it used to
 * take — `onRetry` only means anything in the error branch and `chartData`
 * only in the ready one, which the props could not say. Derived once, in
 * `RevenueChart`, from the query result.
 */
export type ChartBodyState =
  { status: 'loading' } | { status: 'error'; onRetry: () => void } | { status: 'ready'; chartData: ChartDatum[] };
