import Helper from '@ember/component/helper';

const UNITS = ['Bytes', 'KiB', 'MiB', 'GiB'];

/**
 * Bytes Formatter
 *
 * Usage: {{format-bytes bytes}}
 *
 * Outputs the bytes reduced to the largest supported unit size for which
 * bytes is larger than one.
 */
export function reduceToLargestUnit(bytes) {
  bytes || (bytes = 0);
  let unitIndex = 0;
  while (bytes >= 1024 && unitIndex < UNITS.length - 1) {
    bytes /= 1024;
    unitIndex++;
  }

  return [bytes, UNITS[unitIndex]];
}

export function formatBytes([bytes]) {
  const [number, unit] = reduceToLargestUnit(bytes);
  return `${Math.floor(number)} ${unit}`;
}

export default Helper.helper(formatBytes);
