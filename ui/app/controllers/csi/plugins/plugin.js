/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';

export default class CsiPluginsPluginController extends Controller {
  get plugin() {
    return this.model;
  }

  get breadcrumbs() {
    const { plainId } = this.plugin;
    return [
      {
        label: 'Plugins',
        args: ['csi.plugins'],
      },
      {
        label: plainId,
        args: ['csi.plugins.plugin', plainId],
      },
    ];
  }
}
