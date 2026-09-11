/**
 * Monotone cubic Catmull-Rom interpolation.
 * Returns the (x, y) point on the curve at parameter t ∈ [0, 1] between
 * points p1 and p2, using p0 and p3 as surrounding control points.
 */
export function catmullRom(
  p0: [number, number],
  p1: [number, number],
  p2: [number, number],
  p3: [number, number],
  t: number,
): [number, number] {
  const t2 = t * t;
  const t3 = t2 * t;
  const out: [number, number] = [0, 0];
  for (let i = 0; i < 2; i++) {
    const p0i = p0[i];
    const p1i = p1[i];
    const p2i = p2[i];
    const p3i = p3[i];
    // `i` is always 0 or 1 here, both valid indices into these fixed
    // 2-tuples, so this guard never actually skips a value for any real
    // caller — it only satisfies `noUncheckedIndexedAccess`'s inability to
    // prove that for a non-literal index, without asserting with `!`.
    if (p0i === undefined || p1i === undefined || p2i === undefined || p3i === undefined) {
      continue;
    }
    const a = -0.5 * p0i + 1.5 * p1i - 1.5 * p2i + 0.5 * p3i;
    const b = p0i - 2.5 * p1i + 2 * p2i - 0.5 * p3i;
    const c = -0.5 * p0i + 0.5 * p2i;
    const d = p1i;
    out[i] = a * t3 + b * t2 + c * t + d;
  }
  return out;
}
