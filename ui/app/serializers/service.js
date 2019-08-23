import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  attrs: {
    connect: 'Connect',
  },

  normalize(typeHash, hash) {
    if (!hash.Tags) {
      hash.Tags = [];
    }

    return this._super(typeHash, hash);
  },
});
