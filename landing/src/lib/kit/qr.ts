/**
 * The QR code of a complex's public URL, as a module matrix and as an SVG picture. It draws the
 * code and decodes nothing. Built on qrcode-generator with error correction M.
 */
import qrcode from 'qrcode-generator';

/** The blank border a scanner needs around the code, in modules. */
const QUIET_ZONE = 4;

/** The modules of the code for the text, row by row: true is a dark module. */
export function qrMatrix(text: string): boolean[][] {
  const code = qrcode(0, 'M');
  code.addData(text);
  code.make();
  const count = code.getModuleCount();
  return Array.from({ length: count }, (_, row) => Array.from({ length: count }, (_, col) => code.isDark(row, col)));
}

/** The dark modules as one SVG path of unit squares, moved inside the quiet zone. */
function modulePath(matrix: readonly (readonly boolean[])[]): string {
  return matrix
    .flatMap((row, y) => row.flatMap((dark, x) => (dark ? [`M${x + QUIET_ZONE} ${y + QUIET_ZONE}h1v1h-1z`] : [])))
    .join('');
}

/** The code as an SVG picture in a base64 data URL. The picture carries its own quiet zone. */
export function qrSvgDataUrl(text: string, color = '#0b0b0b'): string {
  const matrix = qrMatrix(text);
  const size = matrix.length + QUIET_ZONE * 2;
  const open = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${size} ${size}">`;
  const svg = `${open}<path fill="${color}" d="${modulePath(matrix)}"/></svg>`;
  return `data:image/svg+xml;base64,${btoa(svg)}`;
}
