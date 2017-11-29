import Ember from 'ember';
import ApplicationSerializer from './application';

const { inject, get, assign } = Ember;

export default ApplicationSerializer.extend({
  system: inject.service(),

  normalize(typeHash, hash) {
    hash.FailedTGAllocs = Object.keys(hash.FailedTGAllocs || {}).map(key => {
      return assign({ Name: key }, get(hash, `FailedTGAllocs.${key}`) || {});
    });

    hash.PlainJobId = hash.JobID;
    hash.Namespace =
      hash.Namespace ||
      get(hash, 'Job.Namespace') ||
      this.get('system.activeNamespace.id') ||
      'default';
    hash.JobID = JSON.stringify([hash.JobID, hash.Namespace]);

    // TEMPORARY: https://github.com/emberjs/data/issues/5209
    hash.OriginalJobId = hash.JobID;

    return this._super(typeHash, hash);
  },
});
