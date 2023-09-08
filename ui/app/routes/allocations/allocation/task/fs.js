/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import notifyError from 'nomad-ui/utils/notify-error';

export default class FsRoute extends Route {
  model({ path = '/' }) {
    const decodedPath = decodeURIComponent(path);
    const taskState = this.modelFor('allocations.allocation.task');

    if (!taskState || !taskState.allocation) return;

    const allocation = taskState.allocation;
    const pathWithTaskName = `${taskState.name}${
      decodedPath.startsWith('/') ? '' : '/'
    }${decodedPath}`;

    return RSVP.all([
      allocation.stat(pathWithTaskName),
      taskState.get('allocation.node'),
    ])
      .then(([statJson]) => {
        if (statJson.IsDir) {
          return RSVP.hash({
            path: decodedPath,
            taskState,
            directoryEntries: allocation
              .ls(pathWithTaskName)
              .catch(notifyError(this)),
            isFile: false,
          });
        } else {
          return {
            path: decodedPath,
            taskState,
            isFile: true,
            stat: statJson,
          };
        }
      })
      .catch(notifyError(this));
  }

  setupController(
    controller,
    { path, taskState, directoryEntries, isFile, stat } = {}
  ) {
    super.setupController(...arguments);
    controller.setProperties({
      path,
      taskState,
      directoryEntries,
      isFile,
      stat,
    });
  }
}
