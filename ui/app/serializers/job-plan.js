import { assign } from '@ember/polyfills';
import ApplicationSerializer from './application';
import { get } from '@ember/object';

export default ApplicationSerializer.extend({
  normalize(typeHash, hash) {
    const failures = hash.FailedTGAllocs || {};
    hash.FailedTGAllocs = Object.keys(failures).map(key => {
      return assign({ Name: key }, failures[key] || {});
    });
    hash.PreemptionIDs = (get(hash, 'Annotations.PreemptedAllocs') || []).mapBy('ID');
    return this._super(...arguments);
  },
});
