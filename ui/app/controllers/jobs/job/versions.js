/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { alias } from '@ember/object/computed';
import { action, computed } from '@ember/object';
import classic from 'ember-classic-decorator';

const alertClassFallback = 'is-info';

const errorLevelToAlertClass = {
  danger: 'is-danger',
  warn: 'is-warning',
};

@classic
export default class VersionsController extends Controller.extend(
  WithNamespaceResetting
) {
  error = null;

  @alias('model') job;

  @computed('error.level')
  get errorLevelClass() {
    return (
      errorLevelToAlertClass[this.get('error.level')] || alertClassFallback
    );
  }

  onDismiss() {
    this.set('error', null);
  }

  @action
  handleError(errorObject) {
    this.set('error', errorObject);
  }
}
