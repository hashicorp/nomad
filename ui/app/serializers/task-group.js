import { copy } from 'ember-copy';
import ApplicationSerializer from './application';

export default class TaskGroup extends ApplicationSerializer {
  arrayNullOverrides = ['Services'];
  mapToArray = ['Volumes'];

  normalize(typeHash, hash) {
    // Provide EphemeralDisk to each task
    hash.Tasks.forEach(task => {
      task.EphemeralDisk = copy(hash.EphemeralDisk);
    });

    hash.ReservedEphemeralDisk = hash.EphemeralDisk.SizeMB;

    return super.normalize(typeHash, hash);
  }
}
