import { get } from '@ember/object';
import ApplicationSerializer from './application';

export default class JobSummary extends ApplicationSerializer {
  mapToArray = [
    {
      APIName: 'Summary',
      UIName: 'TaskGroupSummaries',
      convertor: (apiHash, uiHash) => {
        Object.keys(apiHash).forEach(allocKey => (uiHash[`${allocKey}Allocs`] = apiHash[allocKey]));
      },
    },
  ];

  normalize(modelClass, hash) {
    hash.PlainJobId = hash.JobID;
    hash.ID = JSON.stringify([hash.JobID, hash.Namespace || 'default']);
    hash.JobID = hash.ID;

    // Lift the children stats out of the Children object
    const childrenStats = get(hash, 'Children');
    if (childrenStats) {
      Object.keys(childrenStats).forEach(
        childrenKey => (hash[`${childrenKey}Children`] = childrenStats[childrenKey])
      );
    }

    return super.normalize(modelClass, hash);
  }
}
