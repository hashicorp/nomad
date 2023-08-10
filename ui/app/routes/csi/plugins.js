/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

export default class PluginsRoute extends Route.extend(WithForbiddenState) {
  @service store;

  model() {
    return this.store
      .query('plugin', { type: 'csi' })
      .catch(notifyForbidden(this));
  }
}
