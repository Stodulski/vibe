/**
 * A stock adjustment's reason (owner decision, `odd/tasks/pos-cashbox.md`:
 * "reasons rotura, vencimiento, consumo propio, corrección de conteo, otro").
 * Mirrors the `CHECK` constraint on `stock_movements.reason`
 * (`db/migrations/004_pos_catalog_stock.sql`) and the `openapi.yaml` enum.
 */
export const ADJUSTMENT_REASONS = ['breakage', 'expired', 'own_consumption', 'count_correction', 'other'] as const;

export type AdjustmentReason = (typeof ADJUSTMENT_REASONS)[number];
