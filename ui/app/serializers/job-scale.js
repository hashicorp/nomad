import { assign } from '@ember/polyfills';
import ApplicationSerializer from './application';

export default class JobScale extends ApplicationSerializer {
  normalize(modelClass, hash) {
    // Transform the map-based TaskGroups object into an array-based
    // TaskGroupScale fragment list
    hash.PlainJobId = hash.JobID;
    hash.ID = JSON.stringify([hash.JobID, hash.Namespace || 'default']);
    hash.JobID = hash.ID;

    const taskGroups = hash.TaskGroups || {};
    hash.TaskGroupScales = Object.keys(taskGroups).map(key => {
      return assign(taskGroups[key], { Name: key });
    });

    return super.normalize(modelClass, hash);
  }
}
