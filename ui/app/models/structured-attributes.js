/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { set } from '@ember/object';
import { get, computed } from '@ember/object';
import { attr } from '@ember-data/model';
import Fragment from 'ember-data-model-fragments/fragment';
import PathTree from 'nomad-ui/utils/path-tree';

export default class StructuredAttributes extends Fragment {
  @attr() raw;

  recomputeRawProperties(incoming) {
    set(this, 'raw', incoming);
  }

  @computed('raw')
  get structured() {
    const original = this.raw;

    if (!original) {
      return undefined;
    }

    const attrs = Object.keys(original)
      .sort()
      .reduce((obj, key) => {
        obj[key] = original[key];
        return obj;
      }, {});
    let pathValueArray = Object.entries(attrs).map(([path, value]) => {
      return { path, value };
    });
    let pathTree = new PathTree(pathValueArray, { delimiter: '.' });
    return pathTree;
  }

  unknownProperty(key) {
    // Returns the exact value in index 0 and the subtree in index 1
    //
    // ex: nodeAttrs.get('driver.docker')
    // [ "1", { version: "17.05.0-ce", volumes: { enabled: "1" } } ]
    if (this.structured) {
      return get(this.structured, key);
    }
  }
}
