import ApplicationSerializer from './application';
import isIp from 'is-ip';

export default class PortSerializer extends ApplicationSerializer {
  attrs = {
    hostIp: 'HostIP',
  };

  normalize(typeHash, hash) {
    const ip = hash.HostIP;

    if (isIp.v6(ip)) {
      hash.HostIP = `[${ip}]`;
    }

    return super.normalize(...arguments);
  }
}
