/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { computed } from '@ember/object';
import { guidFor } from '@ember/object/internals';

// An Ember.Computed property for creating a unique string with a
// common prefix (based on the guid of the object with the property)
//
// ex. @uniquely('name') // 'name-ember129383'
export default function uniquely(prefix) {
  return computed(function () {
    return `${prefix}-${guidFor(this)}`;
  });
}
