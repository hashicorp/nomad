/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';

export default class IndexRoute extends Route {
  setupController(controller, model) {
    // Suppress the preemptedByAllocation fetch error in the event it's a 404
    if (model) {
      const setPreempter = () =>
        controller.set('preempter', model.preemptedByAllocation);
      model.preemptedByAllocation.then(setPreempter, setPreempter);
    }

    return super.setupController(...arguments);
  }

  resetController(controller, isExiting) {
    if (isExiting) {
      controller.watchNext.cancelAll();
    }
  }
}
