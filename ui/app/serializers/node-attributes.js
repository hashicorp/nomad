import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  normalize(typeHash, hash) {
    return this._super(typeHash, { Attributes: hash });
  },
});
