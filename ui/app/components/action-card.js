/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';

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
      case 'error':
        return 'critical';
      default:
        return 'neutral';
    }
  }

  @action stop() {
    this.instance.socket.close();
  }

  @action stopAll() {
    this.nomadActions.stopPeers(this.instance.peerID);
  }

  @tracked selectedPeer = null;

  @action selectPeer(peer) {
    this.selectedPeer = peer;
  }

  get instance() {
    // Either the passed instance, or the peer-selected instance
    return this.selectedPeer || this.args.instance;
  }

  @tracked hasBeenAnchored = false;

  /**
   * Runs from the action-card template whenever instance.messages updates,
   * and serves to keep the user's view anchored to the bottom of the messages.
   * This uses a hidden element and the overflow-anchor css attribute, which
   * keeps the element visible within the scrollable <code> block parent.
   * A trick here is that, if the user scrolls up from the bottom of the block,
   * we don't want to force them down to the bottom again on update, but we do
   * want to keep them there by default (so they have the latest output).
   * The hasBeenAnchored flag is used to track this state, and we do a little
   * trick when the messages get long enough to cause a scroll to start the
   * anchoring process here.
   *
   * @param {HTMLElement} element
   */
  @action anchorToBottom(element) {
    if (this.hasBeenAnchored) return;
    const parentHeight = element.parentElement.clientHeight;
    const elementHeight = element.clientHeight;
    if (elementHeight > parentHeight) {
      this.hasBeenAnchored = true;
      element.parentElement.scroll(0, elementHeight);
    }
  }
}
