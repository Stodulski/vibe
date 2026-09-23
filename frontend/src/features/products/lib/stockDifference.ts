/**
 * The signed delta the adjustments API wants (`quantity`), derived from what
 * the owner actually types — how many units they counted — rather than
 * asking them to compute a signed number by hand. `undefined` while the
 * field is blank/invalid, so the dialog can tell "nothing typed yet" apart
 * from "typed and the difference happens to be 0".
 */
export function stockDifference(counted: number | undefined, currentStock: number): number | undefined {
  if (counted === undefined || Number.isNaN(counted)) return undefined;
  return counted - currentStock;
}
