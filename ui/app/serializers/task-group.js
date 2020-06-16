import { copy } from 'ember-copy';
import ApplicationSerializer from './application';

export default class TaskGroup extends ApplicationSerializer {
  normalize(typeHash, hash) {
    // Provide EphemeralDisk to each task
    hash.Tasks.forEach(task => {
      task.EphemeralDisk = copy(hash.EphemeralDisk);
    });

    hash.ReservedEphemeralDisk = hash.EphemeralDisk.SizeMB;
    hash.Services = hash.Services || [];

    const volumes = hash.Volumes || {};
    hash.Volumes = Object.keys(volumes).map(key => volumes[key]);

    return super.normalize(typeHash, hash);
  }
}
