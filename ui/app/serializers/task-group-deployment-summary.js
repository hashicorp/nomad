import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  normalize(typeHash, hash) {
    hash.PlacedCanaryAllocations = hash.PlacedCanaries || [];
    delete hash.PlacedCanaries;
    return this._super(typeHash, hash);
  },
});
