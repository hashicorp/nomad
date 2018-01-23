import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  normalize(typeHash, hash) {
    hash.ID = hash.Name;
    return this._super(typeHash, hash);
  },
});
