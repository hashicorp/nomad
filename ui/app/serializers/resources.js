import ApplicationSerializer from './application';

export default class ResourcesSerializer extends ApplicationSerializer {
  attrs = {
    cpu: 'CPU',
    memory: 'MemoryMB',
    disk: 'DiskMB',
    iops: 'IOPS',
  };

  arrayNullOverrides = ['Ports'];
}
