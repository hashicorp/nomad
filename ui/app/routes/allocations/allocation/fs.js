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
    const allocation = this.modelFor('allocations.allocation');

    if (!allocation) return;

    return RSVP.all([allocation.stat(decodedPath), allocation.get('node')])
      .then(([statJson]) => {
        if (statJson.IsDir) {
          return RSVP.hash({
            path: decodedPath,
            allocation,
            directoryEntries: allocation
              .ls(decodedPath)
              .catch(notifyError(this)),
            isFile: false,
          });
        } else {
          return {
            path: decodedPath,
            allocation,
            isFile: true,
            stat: statJson,
          };
        }
      })
      .catch(notifyError(this));
  }

  setupController(
    controller,
    { path, allocation, directoryEntries, isFile, stat } = {}
  ) {
    super.setupController(...arguments);
    controller.setProperties({
      path,
      allocation,
      directoryEntries,
      isFile,
      stat,
    });
  }
}
