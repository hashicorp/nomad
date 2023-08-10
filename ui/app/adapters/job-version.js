/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationAdapter from './application';
import addToPath from 'nomad-ui/utils/add-to-path';
import classic from 'ember-classic-decorator';

@classic
export default class JobVersionAdapter extends ApplicationAdapter {
  revertTo(jobVersion) {
    const jobAdapter = this.store.adapterFor('job');

    const url = addToPath(
      jobAdapter.urlForFindRecord(jobVersion.get('job.id'), 'job'),
      '/revert'
    );
    const [jobName] = JSON.parse(jobVersion.get('job.id'));

    return this.ajax(url, 'POST', {
      data: {
        JobID: jobName,
        JobVersion: jobVersion.number,
      },
    });
  }
}
