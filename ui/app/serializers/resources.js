import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  attrs: {
    cpu: 'CPU',
    memory: 'MemoryMB',
    disk: 'DiskMB',
    iops: 'IOPS',
  },
});
