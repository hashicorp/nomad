import isIp from 'is-ip';

export default function formatHost(address, port) {
  if (!address || !port) {
    return undefined;
  }

  if (isIp.v6(address)) {
    return `[${address}]:${port}`;
  } else {
    return `${address}:${port}`;
  }
}
