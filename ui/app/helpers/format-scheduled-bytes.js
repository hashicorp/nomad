import Helper from '@ember/component/helper';
import { formatScheduledBytes } from 'nomad-ui/utils/units';

/**
 * Scheduled Bytes Formatter
 *
 * Usage: {{format-scheduled-bytes bytes}}
 *
 * Outputs the bytes reduced to the resolution the scheduler
 * and job spec operate at.
 */
function formatScheduledBytesHelper([bytes]) {
  return formatScheduledBytes(bytes);
}

export default Helper.helper(formatScheduledBytesHelper);
