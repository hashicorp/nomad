/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import notifyError from 'nomad-ui/utils/notify-error';

export default class PluginRoute extends Route {
  @service store;
  @service system;

  serialize(model) {
    return { plugin_name: model.get('plainId') };
  }

  model(params) {
    return this.store
      .findRecord('plugin', `csi/${params.plugin_name}`)
      .catch(notifyError(this));
  }
}
