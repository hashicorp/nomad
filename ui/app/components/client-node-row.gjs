/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { macroCondition, isTesting } from '@embroider/macros';
import { capitalize } from '@ember/string';
import didUpdate from '@ember/render-modifiers/modifiers/did-update';
import { LinkTo } from '@ember/routing';
import { on } from '@ember/modifier';
import { service } from '@ember/service';
import { watchRelationship } from 'nomad-ui/utils/properties/watch';
import {
  HdsBadge,
  HdsIcon,
} from '@hashicorp/design-system-components/components';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';

export default class ClientNodeRow extends Component {
  @service store;

  constructor() {
    super(...arguments);

    if (!macroCondition(isTesting())) {
      this._visibilityHandler = this.visibilityHandler.bind(this);
      document.addEventListener('visibilitychange', this._visibilityHandler);
    }

    this.handleNodeChange();
  }

  willDestroy() {
    this.watch.cancelAll();
    if (!macroCondition(isTesting())) {
      document.removeEventListener('visibilitychange', this._visibilityHandler);
    }
    super.willDestroy(...arguments);
  }

  click = (event) => {
    lazyClick([this.args.onClick, event]);
  };

  handleNodeChange = () => {
    const node = this.args.node;
    if (node) {
      node.reload().then(() => {
        this.watch.perform(node, 100);
      });
    }
  };

  visibilityHandler() {
    if (document.hidden) {
      this.watch.cancelAll();
    } else {
      const node = this.args.node;
      if (node) {
        this.watch.perform(node, 100);
      }
    }
  }

  @watchRelationship('allocations') watch;

  get nodeStatusColor() {
    const status = this.args.node?.status;
    if (status === 'disconnected') {
      return 'warning';
    } else if (status === 'down') {
      return 'critical';
    } else if (status === 'ready') {
      return 'success';
    } else if (status === 'initializing') {
      return 'neutral';
    }

    return 'neutral';
  }

  get nodeStatusIcon() {
    const status = this.args.node?.status;
    if (status === 'disconnected') {
      return 'skip';
    } else if (status === 'down') {
      return 'x-circle';
    } else if (status === 'ready') {
      return 'check-circle';
    } else if (status === 'initializing') {
      return 'entry-point';
    }

    return '';
  }

  get nodeStatusText() {
    return capitalize(this.args.node?.status || '');
  }

  <template>
    <tr
      class="client-node-row is-interactive"
      {{on "click" this.click}}
      {{didUpdate this.handleNodeChange @node}}
      ...attributes
    >
      <td data-test-icon class="is-narrow">
        {{#if @node.unhealthyDrivers.length}}
          <span
            class="tooltip text-center"
            role="tooltip"
            aria-label="Client has unhealthy drivers"
          >
            <HdsIcon
              @name="alert-triangle-fill"
              @color="warning"
              @isInline={{true}}
              class="icon-vertical-bump-down"
            />
          </span>
        {{/if}}
      </td>
      <td data-test-client-id><LinkTo
          @route="clients.client"
          @model={{@node.id}}
          class="is-primary"
        >{{@node.shortId}}</LinkTo></td>
      <td
        data-test-client-name
        class="is-200px is-truncatable"
        title="{{@node.name}}"
      >{{@node.name}}</td>
      <td class="node-status-badges" data-test-client-composite-status>
        <HdsBadge
          @text={{this.nodeStatusText}}
          @icon={{this.nodeStatusIcon}}
          @color={{this.nodeStatusColor}}
          @size="small"
        />

        {{#if @node.isEligible}}
          <HdsBadge @text="Eligible" @color="neutral" @size="small" />
        {{else}}
          <HdsBadge @text="Ineligible" @color="neutral" @size="small" />
        {{/if}}

        {{#if @node.isDraining}}
          <HdsBadge @text="Draining" @color="neutral" @size="small" />
        {{else}}
          <HdsBadge @text="Not Draining" @color="neutral" @size="small" />
        {{/if}}
      </td>
      <td
        data-test-client-address
        class="is-200px is-truncatable"
      >{{@node.httpAddr}}</td>
      <td data-test-client-node-pool title="{{@node.nodePool}}">
        {{#if @node.nodePool}}{{@node.nodePool}}{{else}}-{{/if}}
      </td>
      <td data-test-client-datacenter>{{@node.datacenter}}</td>
      <td data-test-client-version>{{@node.version}}</td>
      <td data-test-client-volumes>{{if
          @node.hostVolumes.length
          @node.hostVolumes.length
        }}</td>
      <td data-test-client-allocations>
        {{#if @node.allocations.isPending}}
          ...
        {{else}}
          {{@node.runningAllocations.length}}
        {{/if}}
      </td>
    </tr>
  </template>
}
