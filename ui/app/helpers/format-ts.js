import moment from 'moment';
import Helper from '@ember/component/helper';

export function formatTs([date]) {
  const format = "MMM DD, 'YY HH:mm:ss ZZ";
  return moment(date).format(format);
}

export default Helper.helper(formatTs);
