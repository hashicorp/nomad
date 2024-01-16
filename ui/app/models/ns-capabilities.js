/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Fragment from 'ember-data-model-fragments/fragment';
import { array } from 'ember-data-model-fragments/attributes';

export default class NamespaceCapabilities extends Fragment {
  @array('string') DisabledTaskDrivers;
  @array('string') EnabledTaskDrivers;
}
