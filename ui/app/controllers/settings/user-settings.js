/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

export default class SettingsUserSettingsController extends Controller {
  @localStorageProperty('nomadShouldWrapCode', false) wordWrap;
  @localStorageProperty('nomadLiveUpdateJobsIndex', true) liveUpdateJobsIndex;
}
