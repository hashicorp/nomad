/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import { inject as service } from '@ember/service';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

export default class VariablesVariableRoute extends Route.extend(
  withForbiddenState
) {
  @service store;
  model(params) {
    return this.store
      .findRecord('variable', decodeURIComponent(params.id), {
        reload: true,
      })
      .catch(notifyForbidden(this));
  }
  setupController(controller) {
    super.setupController(controller);
    controller.set('params', this.paramsFor('variables.variable'));
  }
}
