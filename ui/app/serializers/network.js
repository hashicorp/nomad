import ApplicationSerializer from './application';
import isIp from 'is-ip';

export default ApplicationSerializer.extend({
  attrs: {
    cidr: 'CIDR',
    ip: 'IP',
    mbits: 'MBits',
  },

  normalize(typeHash, hash) {
    const ip = hash.IP;

    if (isIp.v6(ip)) {
      hash.IP = `[${ip}]`;
    }

    return this._super(...arguments);
  },
});
