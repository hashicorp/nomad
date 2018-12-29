import Ember from 'ember';

const { Helper } = Ember;

const UNITS = ['Bytes', 'KiB', 'MiB'];

/**
 * Bytes Formatter
 *
 * Usage: {{format-bytes bytes}}
 *
 * Outputs the bytes reduced to the largest supported unit size for which
 * bytes is larger than one.
 */
export function formatBytes([bytes]) {
  let unitIndex = 0;
  while (bytes >= 1024 && unitIndex < UNITS.length - 1) {
    bytes /= 1024;
    unitIndex++;
  }

  return `${Math.floor(bytes)} ${UNITS[unitIndex]}`;
}

export default Helper.helper(formatBytes);
