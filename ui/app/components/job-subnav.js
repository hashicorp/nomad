/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { inject as service } from '@ember/service';
import Component from '@glimmer/component';

export default class JobSubnav extends Component {
  @service can;
  @service keyboard;

  get shouldRenderClientsTab() {
    const { job } = this.args;
    return (
      job?.hasClientStatus && !job?.hasChildren && this.can.can('read client')
    );
  }
}
