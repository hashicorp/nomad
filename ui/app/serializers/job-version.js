/**
 * Copyright IBM Corp. 2015, 2025
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
      });

      // Versions are loaded from a parent job.hasMany("versions") request,
      // so omit ambiguous JobID payload data and let Ember Data bind back to parent.
      delete normalizedVersion.JobID;

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
