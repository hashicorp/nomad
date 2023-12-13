/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { assign } from '@ember/polyfills';
import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class JobVersionSerializer extends ApplicationSerializer {
  attrs = {
    number: 'Version',
  };

  normalizeFindHasManyResponse(store, modelClass, hash, id, requestType) {
    const zippedVersions = hash.Versions.map((version, index) =>
      assign({}, version, {
        Diff: hash.Diffs && hash.Diffs[index],
        ID: `${version.ID}-${version.Version}`,
        JobID: JSON.stringify([version.ID, version.Namespace || 'default']),
        SubmitTime: Math.floor(version.SubmitTime / 1000000),
        SubmitTimeNanos: version.SubmitTime % 1000000,
      })
    );
    return super.normalizeFindHasManyResponse(
      store,
      modelClass,
      zippedVersions,
      hash,
      id,
      requestType
    );
  }
}
