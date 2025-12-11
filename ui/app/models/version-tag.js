/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Fragment from 'ember-data-model-fragments/fragment';
import { attr } from '@ember-data/model';

export default class VersionTagModel extends Fragment {
  @attr() name;
  @attr() description;
  @attr() taggedTime;
  @attr('number') versionNumber;
  @attr('string') jobName;
}
