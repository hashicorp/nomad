/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class ActionCardComponent extends Component {
  get stateColor() {
    /**
     * @type {import('../models/action-instance').default}
     */
    const instance = this.args.instance;
    switch (instance.state) {
      case 'starting':
        return 'neutral';
      case 'running':
        return 'highlight';
      case 'complete':
        return 'success';
      case 'error': // TODO: handle error type
        return 'critical';
      default:
        return 'neutral';
    }
  }

  @action stop() {
    console.log('instance socket is', this.args.instance.socket);
    this.args.instance.socket.close();
  }
}
