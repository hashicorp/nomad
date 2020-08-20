import ApplicationSerializer from './application';

export default class ResourcesSerializer extends ApplicationSerializer {
  attrs = {
    cpu: 'CPU',
    memory: 'MemoryMB',
    disk: 'DiskMB',
    iops: 'IOPS',
  };

  normalize(typeHash, hash) {
    hash.Ports = hash.Ports || [];
    return super.normalize(typeHash, hash);
  }
}
