/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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

          // ID is used for record tracking within Mirage,
          // but Nomad uses the JobID as the version ID.
          version.ID = version.TempVersionID;
          hash.Versions.push(version);
          return hash;
        },
        { Versions: [], Diffs: [] }
      );
  },
});
