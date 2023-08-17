/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Fragment from 'ember-data-model-fragments/fragment';
import { attr } from '@ember-data/model';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default class HostVolume extends Fragment {
  @fragmentOwner() node;

  @attr('string') name;
  @attr('string') path;
  @attr('boolean') readOnly;
}
