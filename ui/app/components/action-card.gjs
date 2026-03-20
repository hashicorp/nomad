/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { capitalize } from '@ember/string';
import didUpdate from '@ember/render-modifiers/modifiers/did-update';
import { fn } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import { on } from '@ember/modifier';
import { service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { eq } from 'ember-truth-helpers';
import {
  HdsBadge,
  HdsButton,
  HdsButtonSet,
  HdsCopyButton,
  HdsPageHeader,
  HdsReveal,
} from '@hashicorp/design-system-components/components';
import formatTs from 'nomad-ui/helpers/format-ts';

export default class ActionCard extends Component {
  @service nomadActions;

  @tracked selectedPeer = null;
  @tracked hasBeenAnchored = false;

  get instance() {
    return this.selectedPeer || this.args.instance;
  }

  get peers() {
    const peerID = this.instance?.peerID;
    if (!peerID) {
      return [];
    }
    return this.nomadActions.actionsQueue.filter(
      (peer) => peer.peerID === peerID,
    );
  }

  get hasRunningPeers() {
    return this.peers.some((peer) => peer.state === 'running');
  }

  get stateText() {
    return capitalize(this.instance?.state || '');
  }

  get stateColor() {
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

  get completedSecondsFloat() {
    const started = this.instance?.createdAt;
    const ended = this.instance?.completedAt;
    if (!started || !ended) {
      return null;
    }
    return (new Date(ended).getTime() - new Date(started).getTime()) / 1000;
  }

  get completedSecondsInt() {
    const seconds = this.completedSecondsFloat;
    if (seconds == null) {
      return null;
    }
    return Math.trunc(seconds);
  }

  get completedLongerThanOneSecond() {
    return (this.completedSecondsInt ?? 0) > 1;
  }

  stop = () => {
    this.instance.socket.close();
  };

  stopAll = () => {
    this.nomadActions.stopPeers(this.instance.peerID);
  };

  selectPeer = (peer) => {
    this.selectedPeer = peer;
  };

  anchorToBottom = (element) => {
    if (this.hasBeenAnchored) return;
    const parentHeight = element.parentElement.clientHeight;
    const elementHeight = element.clientHeight;
    if (elementHeight > parentHeight) {
      this.hasBeenAnchored = true;
      element.parentElement.scroll(0, elementHeight);
    }
  };

  <template>
    <div class="action-card" ...attributes>
      <HdsPageHeader class="action-card-header" as |PH|>
        <PH.Title>
          <span class="action-card-title">
            <span>{{this.instance.action.name}}</span>
            <LinkTo
              class="job-name"
              @route="jobs.job"
              @model={{this.instance.action.task.taskGroup.job}}
            >
              {{this.instance.action.task.taskGroup.job.name}}
            </LinkTo>
          </span>
          <HdsBadge
            @text={{this.stateText}}
            @color={{this.stateColor}}
            @size="medium"
          />
        </PH.Title>
        <PH.Actions>
          {{#if this.instance.peerID}}
            {{#if (eq this.instance.state "running")}}
              <HdsButton
                @text="Stop"
                @color="critical"
                @size="medium"
                {{on "click" this.stop}}
              />
            {{/if}}
            {{#if this.hasRunningPeers}}
              <HdsButton
                @text="Stop All"
                @color="critical"
                @size="medium"
                {{on "click" this.stopAll}}
              />
            {{else}}
              <HdsButton
                @text="Clear"
                @color="secondary"
                {{on
                  "click"
                  (fn this.nomadActions.clearActionInstance this.instance)
                }}
              />
            {{/if}}
          {{else}}
            {{#if (eq this.instance.state "running")}}
              <HdsButton
                @text="Stop"
                @color="critical"
                @size="medium"
                {{on "click" this.stop}}
              />
            {{else}}
              <HdsButton
                @text="Clear"
                @color="secondary"
                {{on
                  "click"
                  (fn this.nomadActions.clearActionInstance this.instance)
                }}
              />
            {{/if}}
          {{/if}}
        </PH.Actions>
      </HdsPageHeader>

      {{#if this.instance.peerID}}
        <HdsButtonSet class="peers">
          {{#each this.peers as |peer|}}
            <HdsButton
              class="peer"
              @icon={{if (eq peer.state "running") "loading" null}}
              @iconPosition="trailing"
              @text={{peer.allocShortID}}
              @color={{if (eq this.instance.id peer.id) "primary" "secondary"}}
              {{on "click" (fn this.selectPeer peer)}}
            />
          {{/each}}
        </HdsButtonSet>
      {{/if}}

      <div class="messages">
        {{#if this.instance.error}}
          <code><pre>Error: {{this.instance.error}}</pre></code>
        {{/if}}
        {{#if this.instance.messages.length}}
          <code tabindex="0">
            <HdsCopyButton
              class="copy-button"
              @text="Copy"
              @isIconOnly={{true}}
              @textToCopy={{this.instance.messages}}
            />
            <pre {{didUpdate this.anchorToBottom this.instance.messages}}>
              {{this.instance.messages}}
            </pre>
            <div class="anchor" />
          </code>
        {{else}}
          {{#if (eq this.instance.state "complete")}}
            <p class="no-messages">Action completed with no output</p>
          {{/if}}
        {{/if}}
      </div>

      <footer>
        <HdsReveal @text="Action Info">
          <ul>
            <li><span>Task:</span> {{this.instance.action.task.name}}</li>
            <li><span>Job:</span>
              {{this.instance.action.task.taskGroup.job.name}}</li>
            <li><span>Allocation:</span> {{this.instance.allocID}}</li>
            <li><span>Created:</span> {{formatTs this.instance.createdAt}}</li>
            {{#if this.instance.completedAt}}
              {{#if this.completedLongerThanOneSecond}}
                <li>Completed after {{this.completedSecondsInt}} seconds</li>
              {{else}}
                <li>Completed in {{this.completedSecondsFloat}} seconds</li>
              {{/if}}
            {{/if}}
          </ul>
        </HdsReveal>
      </footer>

      {{yield}}
    </div>
  </template>
}
