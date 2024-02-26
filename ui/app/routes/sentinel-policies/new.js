/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import TEMPLATES from 'nomad-ui/utils/default-sentinel-policy-templates';

export default class NewRoute extends Route {
  @service store;

  queryParams = {
    template: {
      refreshModel: true,
    },
  };

  model({ template }) {
    let policy = 'main = rule { false }\n';
    if (template) {
      let matchingTemplate = TEMPLATES.find((t) => t.name == template);
      if (matchingTemplate) {
        policy = matchingTemplate.policy;
      }
    }

    return this.store.createRecord('sentinel-policy', {
      name: '',
      policy,
      enforcementLevel: 'advisory',
      scope: 'submit-job',
    });
  }

  resetController(controller, isExiting) {
    if (isExiting) {
      // If user didn't save, delete the freshly created model
      if (controller.model.isNew) {
        controller.model.destroyRecord();
        controller.set('template', null);
      }
    }
  }
}
