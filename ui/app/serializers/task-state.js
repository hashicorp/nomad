import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  normalize(typeHash, hash) {
    // TODO API: finishedAt is always marshaled as a date even when unset.
    // To simplify things, unset it here when it's the empty date value.
    if (hash.FinishedAt === '0001-01-01T00:00:00Z') {
      hash.FinishedAt = null;
    }

    return this._super(typeHash, hash);
  },
});
