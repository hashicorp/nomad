import Helper from '@ember/component/helper';
import formatDuration from '../utils/format-duration';

function formatDurationHelper([duration], { units }) {
  return formatDuration(duration, units);
}

export default Helper.helper(formatDurationHelper);
