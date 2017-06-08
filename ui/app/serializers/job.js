import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  attrs: {
    parameterized: 'parameterized_job',
  },

  normalize(typeHash, hash) {
    console.log(hash);
    return this._super(typeHash, hash);
  },
});
