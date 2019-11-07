import { copy } from 'ember-copy';
import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  normalize(typeHash, hash) {
    // Provide EphemeralDisk to each task
    hash.Tasks.forEach(task => {
      task.EphemeralDisk = copy(hash.EphemeralDisk);
    });

    hash.ReservedEphemeralDisk = hash.EphemeralDisk.SizeMB;
    hash.Services = hash.Services || [];

    return this._super(typeHash, hash);
  },
});
