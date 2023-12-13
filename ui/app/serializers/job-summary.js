/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { get } from '@ember/object';
import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class JobSummary extends ApplicationSerializer {
  normalize(modelClass, hash) {
    hash.PlainJobId = hash.JobID;
    hash.ID = JSON.stringify([hash.JobID, hash.Namespace || 'default']);
    hash.JobID = hash.ID;

    // Transform the map-based Summary object into an array-based
    // TaskGroupSummary fragment list

    const fullSummary = hash.Summary || {};
    hash.TaskGroupSummaries = Object.keys(fullSummary)
      .sort()
      .map((key) => {
        const allocStats = fullSummary[key] || {};
        const summary = { Name: key };

        Object.keys(allocStats).forEach(
          (allocKey) => (summary[`${allocKey}Allocs`] = allocStats[allocKey])
        );

        return summary;
      });

    // Lift the children stats out of the Children object
    const childrenStats = get(hash, 'Children');
    if (childrenStats) {
      Object.keys(childrenStats).forEach(
        (childrenKey) =>
          (hash[`${childrenKey}Children`] = childrenStats[childrenKey])
      );
    }

    return super.normalize(modelClass, hash);
  }
}
