/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import { inject as service } from '@ember/service';
import RSVP from 'rsvp';
import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchQuery, watchAll } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';

export default class IndexRoute extends Route.extend(
  WithWatchers,
  WithForbiddenState
) {
  @service store;

  queryParams = {
    qpNamespace: {
      refreshModel: true,
    },
  };

  model(params) {
    return RSVP.hash({
      csiVolumes: this.store
        .query('volume', { type: 'csi', namespace: params.qpNamespace })
        .catch(notifyForbidden(this)),
      namespaces: this.store.findAll('namespace'),
      dynamicHostVolumes: this.store
        .query('dynamic-host-volume', {
          type: 'host',
          namespace: params.qpNamespace,
        })
        .catch(notifyForbidden(this)),
    });
  }

  startWatchers(controller) {
    controller.set('namespacesWatch', this.watchNamespaces.perform());
    controller.set(
      'modelWatch',
      this.watchVolumes.perform({
        type: 'csi',
        namespace: controller.qpNamespace,
      })
    );
    controller.set(
      'modelWatch',
      this.watchDynamicHostVolumes.perform({
        type: 'host',
        namespace: controller.qpNamespace,
      })
    );
  }

  @watchQuery('volume') watchVolumes;
  @watchQuery('dynamic-host-volume') watchDynamicHostVolumes;
  @watchAll('namespace') watchNamespaces;
  @collect('watchVolumes', 'watchNamespaces', 'watchDynamicHostVolumes')
  watchers;
}
