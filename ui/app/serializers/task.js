import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  normalize(typeHash, hash) {
    // Lift the reserved resource numbers out of the Resources object
    const resources = hash.Resources;
    if (resources) {
      hash.ReservedMemory = resources.MemoryMB;
      hash.ReservedCPU = resources.CPU;
      hash.ReservedDisk = resources.DiskMB;
      hash.ReservedEphemeralDisk = hash.EphemeralDisk.SizeMB;
    }

    return this._super(typeHash, hash);
  },
});
