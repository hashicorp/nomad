import Helper from '@ember/component/helper';

export function formatJobId([jobId]) {
  return JSON.parse(jobId).join('@');
}

export default Helper.helper(formatJobId);
