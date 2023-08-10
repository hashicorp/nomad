/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { collect } from '@ember/object/computed';
import {
  watchRecord,
  watchNonStoreRecords,
} from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import notifyError from 'nomad-ui/utils/notify-error';
export default class AllocationRoute extends Route.extend(WithWatchers) {
  @service notifications;
  @service router;
  @service store;

  startWatchers(controller, model) {
    if (model) {
      controller.set('watcher', this.watch.perform(model));

      const anyGroupServicesAreNomad = !!model.taskGroup?.services?.filterBy(
        'provider',
        'nomad'
      ).length;

      const anyTaskServicesAreNomad = model.states
        .mapBy('task.services')
        .compact()
        .map((fragmentClass) => fragmentClass.mapBy('provider'))
        .flat()
        .any((provider) => provider === 'nomad');

      // Conditionally Long Poll /checks endpoint if alloc has nomad services
      if (anyGroupServicesAreNomad || anyTaskServicesAreNomad) {
        controller.set(
          'watchHealthChecks',
          this.watchHealthChecks.perform(model, 'getServiceHealth', 2000)
        );
      }
    }
  }

  async model() {
    // Preload the job for the allocation since it's required for the breadcrumb trail
    try {
      const [allocation] = await Promise.all([
        super.model(...arguments),
        this.store.findAll('namespace'),
      ]);
      if (allocation.isPartial) {
        await allocation.reload();
      }
      const jobId = allocation.belongsTo('job').id();
      await this.store.findRecord('job', jobId);
      return allocation;
    } catch (e) {
      const [allocId, transition] = arguments;
      if (e?.errors[0]?.detail === 'alloc not found' && !!transition.from) {
        this.notifications.add({
          title: `Error:  Not Found`,
          message: `Allocation of id:  ${allocId} was not found.`,
          color: 'critical',
          sticky: true,
        });
        this.goBackToReferrer(transition.from.name);
      } else {
        notifyError(this)(e);
      }
    }
  }

  goBackToReferrer(referringRoute) {
    this.router.transitionTo(referringRoute);
  }

  @watchRecord('allocation') watch;
  @watchNonStoreRecords('allocation') watchHealthChecks;

  @collect('watch', 'watchHealthChecks') watchers;
}
