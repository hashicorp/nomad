/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { concat } from '@ember/helper';
import { capitalize } from '@ember/string';
import { LinkTo } from '@ember/routing';
import { on } from '@ember/modifier';
import { service } from '@ember/service';
import { HdsBadge } from '@hashicorp/design-system-components/components';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';

export default class ServerAgentRow extends Component {
  // eslint-disable-next-line ember/no-private-routing-service
  @service('-routing') _router;

  get router() {
    return this._router.router;
  }

  get isActive() {
    const router = this.router;
    const targetURL = router.generate('servers.server', this.args.agent);
    const currentURL = `${router.get('rootURL').slice(0, -1)}${
      router.get('currentURL').split('?')[0]
    }`;

    return currentURL.replace(/%40/g, '@') === targetURL.replace(/%40/g, '@');
  }

  goToAgent = (event) => {
    const transition = () =>
      this.router.transitionTo('servers.server', this.args.agent);
    lazyClick([transition, event]);
  };

  get agentStatusColor() {
    const agentStatus = this.args.agent?.status;
    if (agentStatus === 'alive') {
      return 'success';
    } else if (agentStatus === 'failed') {
      return 'critical';
    } else if (agentStatus === 'leaving') {
      return 'neutral';
    } else if (agentStatus === 'left') {
      return 'neutral';
    } else {
      return '';
    }
  }

  get agentStatusText() {
    return capitalize(this.args.agent?.status || '');
  }

  <template>
    <tr
      class="server-agent-row is-interactive {{if this.isActive 'is-active'}}"
      {{on "click" this.goToAgent}}
      ...attributes
    >
      <td
        data-test-server-name
        {{keyboardShortcut enumerated=true action=this.goToAgent}}
      ><LinkTo
          @route="servers.server"
          @model={{@agent.id}}
          class="is-primary"
        >{{@agent.name}}</LinkTo></td>
      <td data-test-server-status><span>
          <HdsBadge
            @text={{this.agentStatusText}}
            @color={{this.agentStatusColor}}
            @size="large"
          />
        </span></td>
      <td data-test-server-is-leader class="server-status-badges">
        <HdsBadge
          @text={{if
            @agent.isLeader
            (if
              @agent.system.shouldShowRegions
              (concat "True" " (" @agent.region ")")
              "True"
            )
            "False"
          }}
          @icon={{if @agent.isLeader "check-circle" ""}}
          @color={{if @agent.isLeader "success" "neutral"}}
          @size="large"
          class="no-wrap"
        />
      </td>
      <td
        data-test-server-address
        class="is-200px is-truncatable"
      >{{@agent.address}}</td>
      <td data-test-server-port>{{@agent.serfPort}}</td>
      <td data-test-server-datacenter>{{@agent.datacenter}}</td>
      <td data-test-server-version>{{@agent.version}}</td>
    </tr>
  </template>
}
