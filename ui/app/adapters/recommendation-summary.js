/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationAdapter from './application';
import classic from 'ember-classic-decorator';

@classic
export default class RecommendationSummaryAdapter extends ApplicationAdapter {
  pathForType = () => 'recommendations';

  urlForFindAll() {
    const url = super.urlForFindAll(...arguments);
    return `${url}?namespace=*`;
  }

  updateRecord(store, type, snapshot) {
    const url = `${super.urlForCreateRecord(
      'recommendations',
      snapshot,
    )}/apply`;

    const allRecommendationIds = snapshot
      .hasMany('recommendations')
      .map(item => item.id);
    const excludedRecommendationIds = (
      snapshot.hasMany('excludedRecommendations') || []
    ).map(item => item.id);
    const includedRecommendationIds = allRecommendationIds.removeObjects(
      excludedRecommendationIds,
    );

    const data = {
      Apply: includedRecommendationIds,
      Dismiss: excludedRecommendationIds,
    };

    return this.ajax(url, 'POST', { data });
  }
}
