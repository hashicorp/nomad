import moment from 'moment';
import Helper from '@ember/component/helper';

export function formatTs([date]) {
  const format = 'MMMM D, YYYY';
  return moment(date).format(format);
}

export default Helper.helper(formatTs);
