/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
// import { alias } from '@ember/object/computed';

export default class ActionCardComponent extends Component {
  @service nomadActions;
  get stateColor() {
    /**
     * @type {import('../models/action-instance').default}
     */
    const instance = this.instance;
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
    this.instance.socket.close();
  }

  @action stopAll() {
    this.nomadActions.stopAll(this.instance.peerID);
  }

  @tracked selectedPeer = null;

  @action selectPeer(peer) {
    this.selectedPeer = peer;
  }

  get instance() {
    // Either the passed instance, or the peer-selected instance
    return this.selectedPeer || this.args.instance;
  }
}
