/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { computed, get } from '@ember/object';
import { htmlSafe } from '@ember/template';

// An Ember.Computed property for transforming an object into an
// html compatible style attribute
//
// ex. styleProps: { color: '#FF0', border-width: '1px' }
//     styleStr: styleStringProperty('styleProps') // color:#FF0;border-width:1px
export default function styleStringProperty(prop) {
  return computed(prop, function () {
    const styles = get(this, prop);
    let str = '';

    if (styles) {
      str = Object.keys(styles)
        .reduce(function (arr, key) {
          const val = styles[key];
          arr.push(
            key + ':' + (typeof val === 'number' ? val.toFixed(2) + 'px' : val)
          );
          return arr;
        }, [])
        .join(';');
    }

    return htmlSafe(str);
  });
}
