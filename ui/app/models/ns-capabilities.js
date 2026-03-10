/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Fragment from 'ember-data-model-fragments/fragment';
import { array } from 'ember-data-model-fragments/attributes';

export default class NamespaceCapabilities extends Fragment {
  @array('string') DisabledTaskDrivers;
  @array('string') EnabledTaskDrivers;
}
