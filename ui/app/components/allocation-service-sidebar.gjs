/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { LinkTo } from '@ember/routing';
import { on } from '@ember/modifier';
import { hash } from '@ember/helper';
import { eq } from 'ember-truth-helpers';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import onClickOutside from 'ember-click-outside/modifiers/on-click-outside';
import keyboardCommands from 'nomad-ui/helpers/keyboard-commands';
import Tooltip from 'nomad-ui/components/tooltip';
import ListTable from 'nomad-ui/components/list-table';
import ServiceStatusIndicator from 'nomad-ui/components/service-status-indicator';

export default class AllocationServiceSidebar extends Component {
  @service store;
  @service system;

  get isSideBarOpen() {
    return !!this.args.service;
  }

  get keyCommands() {
    return [
      {
        label: 'Close Service Sidebar',
        pattern: ['Escape'],
        action: () => this.args.fns?.closeSidebar?.(),
      },
    ];
  }

  get service() {
    return this.store.query('service-fragment', { refID: this.args.serviceID });
  }

  get address() {
    const port = this.args.allocation?.allocatedResources?.ports?.findBy(
      'label',
      this.args.service?.portLabel,
    );

    if (port) {
      return `${port.hostIp}:${port.value}`;
    }

    return null;
  }

  get aggregateStatus() {
    if (this.args.allocation?.clientStatus !== 'running') return 'Unknown';
    const checks = this.checks?.toArray?.() || this.checks || [];
    return checks.some((check) => check.Status === 'failure')
      ? 'Unhealthy'
      : 'Healthy';
  }

  get isConsulProvider() {
    return this.args.service?.provider === 'consul';
  }

  get showConsulLinkNotice() {
    return this.isConsulProvider && !!this.consulRedirectLink;
  }

  get isUnhealthy() {
    return this.aggregateStatus === 'Unhealthy';
  }

  get isUnknown() {
    return this.aggregateStatus === 'Unknown';
  }

  get consulRedirectLink() {
    const config =
      this.system.agent?.config ?? this.system.agent?.get?.('config');
    return config?.UI?.Consul?.BaseUIURL;
  }

  get checks() {
    if (!this.args.service || !this.args.allocation) return [];
    const allocID = this.args.allocation.id;

    // Our UI checks run every 2 seconds; but a check itself may only update every, say, minute.
    // Therefore, we'll have duplicate checks in a service's healthChecks array.
    // Only get the most recent check for each check.
    return (this.args.service.healthChecks || [])
      .filterBy('Alloc', allocID)
      .sortBy('Timestamp')
      .reverse()
      .uniqBy('Check')
      .sortBy('Check');
  }

  checksForName = (checkName) => {
    const checks = (this.args.service?.healthChecks || []).filter(
      (check) => check.Check === checkName,
    );
    const seenTimestamps = new Set();

    return checks.filter((check) => {
      if (seenTimestamps.has(check.Timestamp)) {
        return false;
      }

      seenTimestamps.add(check.Timestamp);
      return true;
    });
  };

  <template>
    <div
      class="sidebar has-subnav service-sidebar
        {{if this.isSideBarOpen 'open'}}"
      {{onClickOutside @fns.closeSidebar capture=true}}
      ...attributes
    >
      {{#if @service}}
        {{keyboardCommands this.keyCommands}}
        <header class="detail-header">
          <h1 class="title">
            {{@service.name}}
            {{#if (eq this.isConsulProvider false)}}
              <span class="aggregate-status">
                {{#if this.isUnhealthy}}
                  <HdsIcon
                    @name="x-square-fill"
                    @color="#c84034"
                    @isInline={{true}}
                  />
                  Unhealthy
                {{else if this.isUnknown}}
                  <Tooltip
                    @text="The parent allocation for this service is not running"
                    @isFullText={{true}}
                  >
                    <HdsIcon @name="help" @color="#999999" @isInline={{true}} />
                    Health Unknown
                  </Tooltip>
                {{else}}
                  <HdsIcon
                    @name="check-square-fill"
                    @color="#25ba81"
                    @isInline={{true}}
                  />
                  Healthy
                {{/if}}
              </span>
            {{/if}}
          </h1>
          <button
            data-test-close-service-sidebar
            class="button is-borderless"
            type="button"
            {{on "click" @fns.closeSidebar}}
          >
            <HdsIcon @name="x" />
          </button>
        </header>

        <div class="boxed-section is-small">
          <div class="boxed-section-body inline-definitions">
            <span class="label">
              Service Details
            </span>

            <div>
              {{#if @service.connect}}
                <span class="pair">
                  <span class="term">
                    Connect
                  </span>
                  <span>True</span>
                </span>
              {{/if}}
              <span class="pair">
                <span class="term">
                  Allocation
                </span>
                <LinkTo
                  @route="allocations.allocation"
                  @model={{@allocation}}
                  @query={{hash service=""}}
                >
                  {{@allocation.shortId}}
                </LinkTo>
              </span>
              <span class="pair">
                <span class="term">
                  IP Address &amp; Port
                </span>
                <a
                  href="http://{{this.address}}"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {{this.address}}
                </a>
              </span>
              <span class="pair">
                <span class="term">
                  Client
                </span>
                <Tooltip @text={{@allocation.node.name}}>
                  <LinkTo
                    @route="clients.client"
                    @model={{@allocation.node.id}}
                  >
                    {{@allocation.node.shortId}}
                  </LinkTo>
                </Tooltip>
              </span>
              {{#if @service.tags.length}}
                <span class="pair is-wrappable">
                  <span class="term">
                    Tags
                  </span>
                  {{#each @service.tags as |tag|}}
                    <span class="tag is-service">{{tag}}</span>
                  {{/each}}
                  {{#each @service.canary_tags as |tag|}}
                    <span class="tag canary is-service">{{tag}}</span>
                  {{/each}}
                </span>
              {{/if}}
            </div>
          </div>
        </div>
        {{#if this.checks.length}}
          <ListTable class="health-checks" @source={{this.checks}} as |t|>
            <t.head>
              <th class="name">
                Check Name
              </th>
              <th class="status">
                Status
              </th>
              <td class="output">
                Output
              </td>
            </t.head>
            <t.body as |row|>
              <tr data-service-health={{row.model.Status}}>
                <td class="name">
                  <span title={{row.model.Check}}>{{row.model.Check}}</span>
                </td>
                <td class="status">
                  <span>
                    {{#if (eq row.model.Status "success")}}
                      <HdsIcon
                        @name="check-square-fill"
                        @color="#25ba81"
                        @isInline={{true}}
                      />
                      Healthy
                    {{else if (eq row.model.Status "failure")}}
                      <HdsIcon
                        @name="x-square-fill"
                        @color="#c84034"
                        @isInline={{true}}
                      />
                      Unhealthy
                    {{else if (eq row.model.Status "pending")}}
                      Pending
                    {{/if}}
                  </span>
                </td>
                <td class="service-output">
                  <code>
                    {{row.model.Output}}
                  </code>
                </td>
              </tr>
              <tr class="service-status-indicators">
                <td colspan="3">
                  <div>
                    {{#each (this.checksForName row.model.Check) as |check|}}
                      <ServiceStatusIndicator @check={{check}} />
                    {{/each}}
                  </div>
                </td>
              </tr>
            </t.body>
          </ListTable>
        {{/if}}
        {{#if this.isConsulProvider}}
          <table class="table is-fixed connect-info">
            <tbody>
              {{#if @service.onUpdate}}
                <tr>
                  <td><strong>On Update</strong></td>
                  <td>{{@service.onUpdate}}</td>
                </tr>
              {{/if}}
              {{#if @service.connect.sidecarService.proxy.upstreams}}
                <tr>
                  <td><strong>Upstreams</strong></td>
                  <td>
                    {{#each
                      @service.connect.sidecarService.proxy.upstreams
                      as |upstream|
                    }}
                      <span
                        class="tag"
                      >{{upstream.destinationName}}:{{upstream.localBindPort}}</span>
                    {{/each}}
                  </td>
                </tr>
              {{/if}}
            </tbody>
          </table>
        {{/if}}
        {{#if this.showConsulLinkNotice}}
          <div data-test-consul-link-notice class="notification is-info">
            <p>
              Nomad cannot read health check information from Consul services,
              but you can
              <a
                href={{this.consulRedirectLink}}
                target="_blank"
                rel="noopener noreferrer"
              >view this information in your Consul UI</a>.
            </p>
          </div>
        {{/if}}
      {{/if}}
    </div>
  </template>
}
