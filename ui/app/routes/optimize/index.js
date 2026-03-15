/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-controller-access-in-routes */
import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { scheduleOnce } from '@ember/runloop';

export default class OptimizeIndexRoute extends Route {
  @service router;

  activate() {
    // This runs late in the loading lifecycle to ensure .filteredSummaries is populated.
    scheduleOnce('actions', this, () => {
      const summaries = this.controllerFor('optimize').filteredSummaries;

      if (!summaries.length) {
        return;
      }

      const firstSummary = summaries[0];
      this.router
        .replaceWith('optimize.summary', firstSummary.slug, {
          queryParams: {
            namespace: firstSummary.jobNamespace || 'default',
          },
        })
        .catch((error) => {
          if (error?.code !== 'TRANSITION_ABORTED') {
            throw error;
          }
        });
    });
  }
}
