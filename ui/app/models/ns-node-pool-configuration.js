/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { attr } from '@ember-data/model';
import Fragment from 'ember-data-model-fragments/fragment';
import { array } from 'ember-data-model-fragments/attributes';

export default class NamespaceNodePoolConfiguration extends Fragment {
  @attr('string') Default;
  @array('string') Allowed;
  @array('string') Disallowed;
}
