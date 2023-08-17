/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check
import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import classic from 'ember-classic-decorator';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

@classic
export default class JobsRunIndexRoute extends Route {
  @service can;
  @service notifications;
  @service router;
  @service store;
  @service system;

  queryParams = {
    template: {
      refreshModel: true,
    },
  };

  beforeModel(transition) {
    if (
      this.can.cannot('run job', null, {
        namespace: transition.to.queryParams.namespace,
      })
    ) {
      this.router.transitionTo('jobs');
    }
  }

  async model({ template }) {
    try {
      // When jobs are created with a namespace attribute, it is verified against
      // available namespaces to prevent redirecting to a non-existent namespace.
      await this.store.findAll('namespace');

      if (template) {
        const VariableAdapter = this.store.adapterFor('variable');
        const templateRecord = await VariableAdapter.getJobTemplate(template);
        return this.store.createRecord('job', {
          _newDefinition: templateRecord.items.template,
        });
      } else {
        return this.store.createRecord('job');
      }
    } catch (e) {
      this.handle404(e);
    }
  }

  handle404(e) {
    const error404 = e.errors?.find((err) => err.status === 404);
    if (error404) {
      this.notifications.add({
        title: `Error loading job template`,
        message: error404.detail,
        color: 'critical',
        sticky: true,
      });

      return;
    }
    notifyForbidden(this)(e);
  }

  resetController(controller, isExiting) {
    if (isExiting) {
      controller.model?.deleteRecord();
      controller.set('template', null);
    }
  }
}
