import ApplicationSerializer from './application';

export default class ResourcesSerializer extends ApplicationSerializer {
  arrayNullOverrides = ['Ports', 'Networks'];

  normalize(typeHash, hash) {
    hash.Cpu = hash.Cpu && hash.Cpu.CpuShares;

    const memory = hash.Memory;
    hash.Memory = memory && memory.MemoryMB;
    hash.MemoryMax = memory && memory.MemoryMaxMB;

    hash.Disk = hash.Disk && hash.Disk.DiskMB;

    // Networks for ReservedResources is different than for Resources.
    // This smooths over the differences, but doesn't actually support
    // anything in the ReservedResources.Networks object, since we don't
    // use any of it in the UI.
    if (!(hash.Networks instanceof Array)) {
      hash.Networks = [];
    }

    return super.normalize(...arguments);
  }
}
