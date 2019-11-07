import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { assign } from '@ember/polyfills';
import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  system: service(),

  normalize(typeHash, hash) {
    const failures = hash.FailedTGAllocs || {};
    hash.FailedTGAllocs = Object.keys(failures).map(key => {
      const propertiesForKey = failures[key] || {};
      return assign({ Name: key }, propertiesForKey);
    });

    hash.PlainJobId = hash.JobID;
    hash.Namespace =
      hash.Namespace ||
      get(hash, 'Job.Namespace') ||
      this.get('system.activeNamespace.id') ||
      'default';
    hash.JobID = JSON.stringify([hash.JobID, hash.Namespace]);

    hash.ModifyTimeNanos = hash.ModifyTime % 1000000;
    hash.ModifyTime = Math.floor(hash.ModifyTime / 1000000);

    hash.CreateTimeNanos = hash.CreateTime % 1000000;
    hash.CreateTime = Math.floor(hash.CreateTime / 1000000);

    return this._super(typeHash, hash);
  },
});
