/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

export default class SettingsUserSettingsController extends Controller {
  @localStorageProperty('nomadShouldWrapCode', false) wordWrap;
  @localStorageProperty('nomadLiveUpdateJobsIndex', true) liveUpdateJobsIndex;
}
