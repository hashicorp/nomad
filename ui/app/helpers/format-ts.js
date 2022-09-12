import moment from 'moment';
import Helper from '@ember/component/helper';

export function formatTs([date], options = {}) {
  const format = options.short ? 'MMM D' : "MMM DD, 'YY HH:mm:ss ZZ";
  return moment(date).format(format);
}

export default Helper.helper(formatTs);
