/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class JobStatusPanelComponent extends Component {
  @service store;

  get isActivelyDeploying() {
    return this.args.job.get('latestDeployment.isRunning');
  }

  get nodes() {
    if (!this.args.job.get('hasClientStatus')) {
      return [];
    }
    return this.store.peekAll('node');
  }
}
