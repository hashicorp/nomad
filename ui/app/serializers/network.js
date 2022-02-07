import ApplicationSerializer from './application';
import isIp from 'is-ip';

export default class NetworkSerializer extends ApplicationSerializer {
  attrs = {
    cidr: 'CIDR',
    ip: 'IP',
    mbits: 'MBits',
  };

  normalize(typeHash, hash) {
    const ip = hash.IP;

    if (isIp.v6(ip)) {
      hash.IP = `[${ip}]`;
    }

    const reservedPorts = (hash.ReservedPorts || []).map((port) => ({
      name: port.Label,
      port: port.Value,
      to: port.To,
      isDynamic: false,
    }));

    const dynamicPorts = (hash.DynamicPorts || []).map((port) => ({
      name: port.Label,
      port: port.Value,
      to: port.To,
      isDynamic: true,
    }));

    hash.Ports = reservedPorts.concat(dynamicPorts).sortBy('name');

    return super.normalize(...arguments);
  }
}
