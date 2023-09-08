/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Watchable from './watchable';
import addToPath from 'nomad-ui/utils/add-to-path';
import classic from 'ember-classic-decorator';

@classic
export default class AllocationAdapter extends Watchable {
  stop = adapterAction('/stop');

  restart(allocation, taskName) {
    const prefix = `${this.host || '/'}${this.urlPrefix()}`;
    const url = `${prefix}/client/allocation/${allocation.id}/restart`;
    return this.ajax(url, 'PUT', {
      data: taskName && { TaskName: taskName },
    });
  }

  restartAll(allocation) {
    const prefix = `${this.host || '/'}${this.urlPrefix()}`;
    const url = `${prefix}/client/allocation/${allocation.id}/restart`;
    return this.ajax(url, 'PUT', { data: { AllTasks: true } });
  }

  ls(model, path) {
    return this.token
      .authorizedRequest(
        `/v1/client/fs/ls/${model.id}?path=${encodeURIComponent(path)}`
      )
      .then(handleFSResponse);
  }

  stat(model, path) {
    return this.token
      .authorizedRequest(
        `/v1/client/fs/stat/${model.id}?path=${encodeURIComponent(path)}`
      )
      .then(handleFSResponse);
  }

  async check(model) {
    const res = await this.token.authorizedRequest(
      `/v1/client/allocation/${model.id}/checks`
    );
    const data = await res.json();
    // Append allocation ID to each check
    Object.values(data).forEach((check) => {
      check.Alloc = model.id;
      check.Timestamp = check.Timestamp * 1000; // Convert to milliseconds
    });
    return data;
  }
}

async function handleFSResponse(response) {
  if (response.ok) {
    return response.json();
  } else {
    const body = await response.text();

    throw {
      code: response.status,
      toString: () => body,
    };
  }
}

function adapterAction(path, verb = 'POST') {
  return function (allocation) {
    const url = addToPath(
      this.urlForFindRecord(allocation.id, 'allocation'),
      path
    );
    return this.ajax(url, verb);
  };
}
