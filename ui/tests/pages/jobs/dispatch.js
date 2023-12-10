/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  clickable,
  collection,
  create,
  fillable,
  isPresent,
  isVisible,
  property,
  text,
  visitable,
} from 'ember-cli-page-object';
import { codeFillable, code } from 'nomad-ui/tests/pages/helpers/codemirror';

export default create({
  visit: visitable('/jobs/:id/dispatch'),

  dispatchButton: {
    scope: '[data-test-dispatch-button]',
    isDisabled: property('disabled'),
    click: clickable(),
    isPresent: isPresent(),
  },

  hasError: isVisible('[data-test-dispatch-error]'),

  metaFields: collection('[data-test-meta-field]', {
    field: {
      scope: '[data-test-meta-field-input]',
      input: fillable(),
      id: property('id'),
    },
    label: text('[data-test-meta-field-label]'),
  }),

  optionalMetaFields: collection('[data-test-meta-field="optional"]', {
    field: {
      scope: '[data-test-meta-field-input]',
      input: fillable(),
      id: property('id'),
    },
    label: text('[data-test-meta-field-label]'),
  }),

  payload: {
    title: text('[data-test-payload-head]'),
    editor: {
      scope: '[data-test-payload-editor]',
      isPresent: isPresent(),
      contents: code('[data-test-payload-editor]'),
      fillIn: codeFillable('[data-test-payload-editor]'),
    },
    emptyMessage: {
      scope: '[data-test-empty-payload-message]',
      isPresent: isPresent(),
    },
  },
});
