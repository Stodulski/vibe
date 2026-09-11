import type { ChartDatum } from './chart-draw';
import { useCanvasChart } from './useCanvasChart';
import { useChartTooltip } from './useChartTooltip';
import { ChartTooltip } from './ChartTooltip';
import { ChartHoverIndicator } from './ChartHoverIndicator';

export function CanvasAreaChart({ data }: { data: ChartDatum[] }) {
  const { canvasRef, wrapperRef, layoutRef, rectRef, resizeTick } = useCanvasChart(data);
  const { tooltip, tooltipStyle, handlePointerMove, handlePointerLeave } = useChartTooltip(
    data,
    layoutRef,
    rectRef,
    resizeTick,
  );

  return (
    <div ref={wrapperRef} className="relative h-full w-full">
      <canvas
        ref={canvasRef}
        className="h-full w-full"
        onPointerMove={handlePointerMove}
        onPointerLeave={handlePointerLeave}
      />
      {tooltip && tooltipStyle && <ChartTooltip tooltip={tooltip} style={tooltipStyle} />}
      {tooltip && <ChartHoverIndicator x={tooltip.x} y={tooltip.y} lineHeight={tooltip.bottomY} />}
    </div>
  );
}
