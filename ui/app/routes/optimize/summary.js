/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import notifyError from 'nomad-ui/utils/notify-error';

export default class OptimizeSummaryRoute extends Route {
  async model({ namespace, slug }) {
    const selectedNamespace =
      namespace || this.paramsFor('optimize.summary')?.namespace || 'default';

    const model = this.modelFor('optimize').summaries.find(
      (summary) =>
        summary.slug === slug && summary.jobNamespace === selectedNamespace,
    );

    if (!model) {
      const error = new Error(
        `Unable to find summary for ${slug} in namespace ${selectedNamespace}`,
      );
      error.code = 404;
      notifyError(this)(error);
    } else {
      return model;
    }
  }
}
