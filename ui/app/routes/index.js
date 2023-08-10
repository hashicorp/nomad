/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';

export default class IndexRoute extends Route {
  redirect() {
    this.transitionTo('jobs');
  }
}
