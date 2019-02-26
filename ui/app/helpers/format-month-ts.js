import moment from 'moment';
import Helper from '@ember/component/helper';

export function formatMonthTs([date]) {
  const format = 'MMM DD HH:mm:ss ZZ';
  return moment(date).format(format);
}

export default Helper.helper(formatMonthTs);
