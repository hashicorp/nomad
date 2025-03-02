/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import notifyError from 'nomad-ui/utils/notify-error';
import { inject as service } from '@ember/service';

export default class StorageVolumesDynamicHostVolumeRoute extends Route {
  @service store;
  @service system;

  model(params) {
    const [id, namespace] = params.id.split('@');
    const fullId = JSON.stringify([`${id}`, namespace || 'default']);

    return RSVP.hash({
      volume: this.store.findRecord('dynamic-host-volume', fullId, {
        reload: true,
      }),
      namespaces: this.store.findAll('namespace'),
    })
      .then((hash) => hash.volume)
      .catch(notifyError(this));
  }
}
