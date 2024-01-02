/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { getOwner } from '@ember/application';
import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

export default class JobsRunTemplatesNewRoute extends Route {
  @service can;
  @service router;
  @service store;
  @service system;

  beforeModel(transition) {
    if (
      this.can.cannot('write variable', null, {
        namespace: transition.to.queryParams.namespace,
      })
    ) {
      this.router.transitionTo('jobs.run');
    }
  }

  async model() {
    try {
      // When variables are created with a namespace attribute, it is verified against
      // available namespaces to prevent redirecting to a non-existent namespace.
      await Promise.all([
        this.store.query('variable', {
          prefix: 'nomad/job-templates',
          namespace: '*',
        }),
        this.store.findAll('namespace'),
      ]);

      // When navigating from jobs.run.index using "Save as Template"
      const json = getOwner(this).lookup('controller:jobs.run').jsonTemplate;
      if (json) {
        return this.store.createRecord('variable', {
          keyValues: [{ key: 'template', value: json }],
        });
      }

      // Override Default Value
      return this.store.createRecord('variable', { keyValues: [] });
    } catch (e) {
      notifyForbidden(this)(e);
    }
  }

  resetController(controller, isExiting) {
    if (
      isExiting &&
      controller.model.isNew &&
      !controller.model.isDestroyed &&
      !controller.model.isDestroying
    ) {
      controller.model?.unloadRecord();
    }
    controller.set('templateName', null);
    controller.set('templateNamespace', 'default');
    getOwner(this).lookup('controller:jobs.run').jsonTemplate = null;
  }
}
