import Ember from 'ember';
import ApplicationSerializer from './application';

const { assign } = Ember;

export default ApplicationSerializer.extend({
  attrs: {
    number: 'Version',
  },

  normalizeFindHasManyResponse(store, modelClass, hash, id, requestType) {
    const zippedVersions = hash.Versions.map((version, index) =>
      assign({}, version, {
        Diff: hash.Diffs && hash.Diffs[index],
        ID: `${version.ID}-${version.Version}`,
        SubmitTime: Math.floor(version.SubmitTime / 1000000),
        SubmitTimeNanos: version.SubmitTime % 1000000,
      })
    );
    return this._super(store, modelClass, zippedVersions, hash, id, requestType);
  },
});
