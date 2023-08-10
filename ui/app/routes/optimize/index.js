/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-controller-access-in-routes */
import Route from '@ember/routing/route';

export default class OptimizeIndexRoute extends Route {
  async activate() {
    // This runs late in the loading lifecycle to ensure .filteredSummaries is populated
    const summaries = this.controllerFor('optimize').filteredSummaries;

    if (summaries.length) {
      const firstSummary = summaries.objectAt(0);

      return this.transitionTo('optimize.summary', firstSummary.slug, {
        queryParams: {
          jobNamespace: firstSummary.jobNamespace || 'default',
        },
      });
    }
  }
}
