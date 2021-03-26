import Helper from '@ember/component/helper';
import { formatBytes } from 'nomad-ui/utils/units';

/**
 * Bytes Formatter
 *
 * Usage: {{format-bytes bytes}}
 *
 * Outputs the bytes reduced to the largest supported unit size for which
 * bytes is larger than one.
 */
function formatBytesHelper([bytes]) {
  return formatBytes(bytes);
}

export default Helper.helper(formatBytesHelper);
