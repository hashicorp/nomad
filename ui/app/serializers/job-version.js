/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class JobVersionSerializer extends ApplicationSerializer {
  attrs = {
    number: 'Version',
  };

  normalize(typeHash, hash) {
    if (hash.TaggedVersion) {
      hash.TaggedVersion.VersionNumber = hash.Version;
    }
    return super.normalize(typeHash, hash);
  }

  normalizeFindHasManyResponse(store, modelClass, hash, id, requestType) {
    const zippedVersions = hash.Versions.map((version, index) => {
      const normalizedVersion = Object.assign({}, version, {
        Diff: hash.Diffs && hash.Diffs[index],
        ID: `${version.ID}-${version.Version}`,
        SubmitTime: Math.floor(version.SubmitTime / 1000000),
        SubmitTimeNanos: version.SubmitTime % 1000000,
        // Contruct the JobID so version properly links to the job.
        JobID: JSON.stringify([version.ID, version.Namespace || 'default']),
      });

      return normalizedVersion;
    });

    return super.normalizeFindHasManyResponse(
      store,
      modelClass,
      zippedVersions,
      id,
      requestType,
    );
  }
}
