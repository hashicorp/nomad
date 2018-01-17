import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { assign } from '@ember/polyfills';
import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  system: service(),

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
