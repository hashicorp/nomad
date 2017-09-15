import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  attrs: {
    cidr: 'CIDR',
    ip: 'IP',
    mbits: 'MBits',
  },
});
