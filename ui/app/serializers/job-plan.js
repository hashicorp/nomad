import { get } from '@ember/object';
import { assign } from '@ember/polyfills';
import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  normalize(typeHash, hash) {
    hash.FailedTGAllocs = Object.keys(hash.FailedTGAllocs || {}).map(key => {
      return assign({ Name: key }, get(hash, `FailedTGAllocs.${key}`) || {});
    });
    return this._super(...arguments);
  },
});
