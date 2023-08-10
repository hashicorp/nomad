/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory, trait } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';

export default Factory.extend({
  groupNames: [],

  jobId: '',
  JobID() {
    return this.jobId;
  },
  namespace: null,
  shallow: false,

  afterCreate(jobScale, server) {
    const groups = jobScale.groupNames.map(group =>
      server.create('task-group-scale', {
        id: group,
        shallow: jobScale.shallow,
      })
    );

    jobScale.update({
      taskGroupScaleIds: groups.mapBy('id'),
    });
  },
});
