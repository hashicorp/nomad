/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class DasDismissedComponent extends Component {
  @localStorageProperty('nomadRecommendationDismssalUnderstood', false)
  explanationUnderstood;

  @tracked dismissInTheFuture = false;

  @action
  proceedAutomatically() {
    this.args.proceed({ manuallyDismissed: false });
  }

  @action
  understoodClicked() {
    this.explanationUnderstood = this.dismissInTheFuture;
    this.args.proceed({ manuallyDismissed: true });
  }
}
