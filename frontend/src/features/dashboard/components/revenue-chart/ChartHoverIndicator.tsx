import { strokeColor } from './chart-draw';

interface ChartHoverIndicatorProps {
  x: number;
  y: number;
  lineHeight: number;
}

/** Vertical guide line + highlighted dot shown at the hovered data point. */
export function ChartHoverIndicator({ x, y, lineHeight }: ChartHoverIndicatorProps) {
  const color = strokeColor();
  return (
    <>
      <div
        className="pointer-events-none absolute top-0"
        style={{ left: x, height: lineHeight, width: 1, background: 'rgba(255,255,255,0.08)' }}
      />
      <div
        className="pointer-events-none absolute"
        style={{
          left: x - 4,
          top: y - 4,
          width: 8,
          height: 8,
          borderRadius: '50%',
          background: color,
          boxShadow: `0 0 6px ${color}`,
        }}
      />
    </>
  );
}
