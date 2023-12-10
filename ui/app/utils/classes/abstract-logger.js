/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { assert } from '@ember/debug';
import Mixin from '@ember/object/mixin';
import { computed } from '@ember/object';
import { computed as overridable } from 'ember-overridable-computed';
import { assign } from '@ember/polyfills';
import queryString from 'query-string';

const MAX_OUTPUT_LENGTH = 50000;

// eslint-disable-next-line ember/no-new-mixins
export default Mixin.create({
  url: '',
  params: overridable(() => ({})),
  logFetch() {
    assert(
      'Loggers need a logFetch method, which should have an interface like window.fetch'
    );
  },

  endOffset: null,

  offsetParams: computed('endOffset', function () {
    const endOffset = this.endOffset;
    return endOffset
      ? { origin: 'start', offset: endOffset }
      : { origin: 'end', offset: MAX_OUTPUT_LENGTH };
  }),

  additionalParams: overridable(() => ({})),

  fullUrl: computed(
    'url',
    'params',
    'offsetParams',
    'additionalParams',
    function () {
      const queryParams = queryString.stringify(
        assign({}, this.params, this.offsetParams, this.additionalParams)
      );
      return `${this.url}?${queryParams}`;
    }
  ),
});
