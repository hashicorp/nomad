import Helper from '@ember/component/helper';

export function formatPort([hostIp, value]) {
  return `${hostIp}:${value}`;
}

export default Helper.helper(formatPort);
