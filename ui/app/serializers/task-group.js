import Ember from 'ember';
import ApplicationSerializer from './application';

const { copy } = Ember;

export default ApplicationSerializer.extend({
  normalize(typeHash, hash) {
    // Provide EphemeralDisk to each task
    hash.Tasks.forEach(task => {
      task.EphemeralDisk = copy(hash.EphemeralDisk);
    });

    hash.ReservedEphemeralDisk = hash.EphemeralDisk.SizeMB;

    return this._super(typeHash, hash);
  },
});
