/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import classic from 'ember-classic-decorator';
import ApplicationSerializer from './application';

@classic
export default class VariableSerializer extends ApplicationSerializer {
  separateNanos = ['CreateTime', 'ModifyTime'];

  normalize(typeHash, hash) {
    // ID is a composite of both the job ID and the namespace the job is in
    hash.ID = `${hash.Path}@${hash.Namespace || 'default'}`;
    return super.normalize(typeHash, hash);
  }

  // Transform API's Items object into an array of a KeyValue objects
  normalizeFindRecordResponse(store, typeClass, hash, id, ...args) {
    if (!hash.Items) {
      hash.Items = { '': '' };
    }
    hash.KeyValues = Object.entries(hash.Items)
      .map(([key, value]) => {
        return {
          key,
          value,
        };
      })
      .sort((a, b) => a.key.localeCompare(b.key));
    delete hash.Items;
    return super.normalizeFindRecordResponse(
      store,
      typeClass,
      hash,
      id,
      ...args
    );
  }

  normalizeDefaultJobTemplate(hash) {
    return {
      path: hash.id,
      isDefaultJobTemplate: true,
      ...hash,
    };
  }

  // Transform our KeyValues array into an Items object
  serialize(snapshot, options) {
    const json = super.serialize(snapshot, options);
    json.ID = json.Path;
    json.Items = json.KeyValues.reduce((acc, { key, value }) => {
      acc[key] = value;
      return acc;
    }, {});
    delete json.KeyValues;
    delete json.ModifyTime;
    delete json.CreateTime;
    return json;
  }
}
