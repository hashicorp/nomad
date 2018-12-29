import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);

    if (!(json instanceof Array)) {
      json = [json];
    }

    return json
      .sortBy('SubmitTime')
      .reverse()
      .reduce(
        (hash, version) => {
          hash.Diffs.push(version.Diff);
          delete version.Diff;

          hash.Versions.push(version);
          return hash;
        },
        { Versions: [], Diffs: [] }
      );
  },
});
