/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, fn, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import {
  HdsAlert,
  HdsButton,
  HdsCardContainer,
  HdsDropdown,
  HdsFormTextInputField,
  HdsLinkInline,
  HdsPageHeader,
  HdsPaginationNumbered,
  HdsTable,
} from '@hashicorp/design-system-components/components';
import { pageTitle } from 'ember-page-title';
import eq from 'ember-truth-helpers/helpers/eq';
import gt from 'ember-truth-helpers/helpers/gt';
import momentFromNow from 'ember-moment/helpers/moment-from-now';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import ForbiddenMessage from 'nomad-ui/components/forbidden-message';
import StorageSubnav from 'nomad-ui/components/storage-subnav';
import formatMonthTs from 'nomad-ui/helpers/format-month-ts';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';

<template>
  {{pageTitle "Storage"}}

  <Breadcrumb @crumb={{hash label="Storage" args=(array "storage.index")}} />
  {{outlet}}

  <StorageSubnav />
  <section class="section storage-index">
    <HdsPageHeader @title="Storage" as |PH|>
      <PH.Actions>
        {{#if @controller.system.shouldShowNamespaces}}
          <HdsDropdown data-test-namespace-facet as |dd|>
            <dd.ToggleButton
              @text="Namespace ({{@controller.qpNamespace}})"
              @color="secondary"
            />
            {{#each @controller.optionsNamespaces as |option|}}
              <dd.Radio
                name={{option.key}}
                {{on "change" (fn @controller.setNamespace option.key)}}
                checked={{eq @controller.qpNamespace option.key}}
              >
                {{option.label}}
              </dd.Radio>
            {{/each}}
          </HdsDropdown>
        {{/if}}
      </PH.Actions>
    </HdsPageHeader>

    {{#if @controller.isForbidden}}
      <ForbiddenMessage />
    {{else}}
      <HdsCardContainer
        @level="base"
        @hasBorder={{false}}
        class="storage-index-table-card"
        data-test-csi-volumes-card
      >
        <header aria-label="CSI Volumes">
          <h3>CSI Volumes</h3>
          <p class="intro">
            Storage configured by plugins run as Nomad jobs, with advanced
            features like snapshots and resizing.
            <HdsLinkInline
              @href="https://developer.hashicorp.com/nomad/docs/other-specifications/volume/csi"
              @icon="docs-link"
              @iconPosition="trailing"
            >Read more</HdsLinkInline>
          </p>
          <div class="search">
            <HdsFormTextInputField
              data-test-csi-volumes-search
              @type="search"
              @value={{@controller.csiFilter}}
              placeholder="Search CSI Volumes"
              {{on "input" (fn @controller.applyFilter "csi")}}
            />
          </div>
        </header>
        {{#if @controller.sortedCSIVolumes.length}}
          <HdsTable
            @caption="CSI Volumes"
            @model={{@controller.paginatedCSIVolumes}}
            @columns={{@controller.csiColumns}}
            @sortBy={{@controller.csiSortProperty}}
            @sortOrder={{if @controller.csiSortDescending "desc" "asc"}}
            @onSort={{fn @controller.handleSort "csi"}}
          >
            <:body as |B|>
              <B.Tr
                data-test-csi-volume-row
                {{keyboardShortcut
                  enumerated=true
                  action=(fn @controller.openCSI B.data)
                }}
              >
                <B.Td data-test-csi-volume-name>
                  <LinkTo
                    @route="storage.volumes.volume"
                    @model={{B.data.idWithNamespace}}
                    class="is-primary"
                  >
                    {{B.data.plainId}}
                  </LinkTo>
                </B.Td>
                {{#if @controller.system.shouldShowNamespaces}}
                  <B.Td data-test-csi-volume-namespace>
                    {{B.data.namespace.name}}
                  </B.Td>
                {{/if}}
                <B.Td data-test-csi-volume-schedulable>
                  {{if B.data.schedulable "Schedulable" "Unschedulable"}}
                </B.Td>
                <B.Td data-test-csi-volume-controller-health>
                  {{#if B.data.controllerRequired}}
                    {{if
                      (gt B.data.controllersHealthy 0)
                      "Healthy"
                      "Unhealthy"
                    }}
                    (
                    {{B.data.controllersHealthy}}
                    /
                    {{B.data.controllersExpected}}
                    )
                  {{else if (gt B.data.controllersExpected 0)}}
                    {{if
                      (gt B.data.controllersHealthy 0)
                      "Healthy"
                      "Unhealthy"
                    }}
                    (
                    {{B.data.controllersHealthy}}
                    /
                    {{B.data.controllersExpected}}
                    )
                  {{else}}
                    <em class="is-faded">
                      Node Only
                    </em>
                  {{/if}}
                </B.Td>
                <B.Td data-test-csi-volume-node-health>
                  {{if (gt B.data.nodesHealthy 0) "Healthy" "Unhealthy"}}
                  (
                  {{B.data.nodesHealthy}}
                  /
                  {{B.data.nodesExpected}}
                  )
                </B.Td>
                <B.Td data-test-csi-volume-plugin>
                  <LinkTo
                    @route="storage.plugins.plugin"
                    @model={{B.data.plugin.plainId}}
                  >
                    {{B.data.plugin.plainId}}
                  </LinkTo>
                </B.Td>
                <B.Td data-test-csi-volume-allocations>
                  {{B.data.allocationCount}}
                </B.Td>
              </B.Tr>
            </:body>
          </HdsTable>
          <HdsPaginationNumbered
            @totalItems={{@controller.filteredCSIVolumes.length}}
            @currentPage={{@controller.csiPage}}
            @pageSizes={{@controller.pageSizes}}
            @currentPageSize={{@controller.userSettings.pageSize}}
            @onPageChange={{fn @controller.handlePageChange "csi"}}
            @onPageSizeChange={{@controller.setUserPageSize}}
          />
        {{else}}
          <div class="empty-message" data-test-empty-csi-volumes-list-headline>
            {{#if @controller.csiFilter}}
              <p>No CSI volumes match your search for "{{@controller.csiFilter}}"</p>
              <HdsButton
                @text="Clear search"
                @color="secondary"
                {{on "click" (fn @controller.clearFilter "csi")}}
              />
            {{else}}
              <p>No CSI Volumes found</p>
            {{/if}}
          </div>
        {{/if}}
      </HdsCardContainer>

      <HdsCardContainer
        @level="base"
        @hasBorder={{false}}
        class="storage-index-table-card"
        data-test-dynamic-host-volumes-card
      >
        <header aria-label="Dynamic Host Volumes">
          <h3>Dynamic Host Volumes</h3>
          <p class="intro">
            Storage provisioned via plugin scripts on a particular client,
            modifiable without requiring client restart.
            <HdsLinkInline
              @href="https://developer.hashicorp.com/nomad/docs/other-specifications/volume/host"
              @icon="docs-link"
              @iconPosition="trailing"
            >Read more</HdsLinkInline>
          </p>
          <div class="search">
            <HdsFormTextInputField
              data-test-dynamic-host-volumes-search
              @type="search"
              @value={{@controller.dhvFilter}}
              placeholder="Search Dynamic Host Volumes"
              {{on "input" (fn @controller.applyFilter "dhv")}}
            />
          </div>
        </header>
        {{#if @controller.sortedDynamicHostVolumes.length}}
          <HdsTable
            @caption="Dynamic Host Volumes"
            @model={{@controller.paginatedDynamicHostVolumes}}
            @columns={{@controller.dhvColumns}}
            @sortBy={{@controller.dhvSortProperty}}
            @sortOrder={{if @controller.dhvSortDescending "desc" "asc"}}
            @onSort={{fn @controller.handleSort "dhv"}}
          >
            <:body as |B|>
              <B.Tr
                data-test-dhv-row
                {{keyboardShortcut
                  enumerated=true
                  action=(fn @controller.openDHV B.data)
                }}
              >
                <B.Td>
                  <LinkTo
                    data-test-dhv-name={{B.data.name}}
                    @route="storage.volumes.dynamic-host-volume"
                    @model={{B.data.idWithNamespace}}
                  >
                    {{B.data.plainId}}
                  </LinkTo>
                </B.Td>
                <B.Td>
                  {{B.data.name}}
                </B.Td>
                {{#if @controller.system.shouldShowNamespaces}}
                  <B.Td>{{B.data.namespace}}</B.Td>
                {{/if}}
                <B.Td>
                  <LinkTo @route="clients.client" @model={{B.data.node.id}}>
                    {{B.data.node.name}}
                  </LinkTo>
                </B.Td>
                <B.Td>{{B.data.pluginID}}</B.Td>
                <B.Td>{{B.data.state}}</B.Td>
                <B.Td>
                  <span
                    class="tooltip"
                    aria-label={{formatMonthTs B.data.modifyTime}}
                  >
                    {{momentFromNow B.data.modifyTime}}
                  </span>
                </B.Td>
              </B.Tr>
            </:body>
          </HdsTable>
          <HdsPaginationNumbered
            @totalItems={{@controller.filteredDynamicHostVolumes.length}}
            @currentPage={{@controller.dhvPage}}
            @pageSizes={{@controller.pageSizes}}
            @currentPageSize={{@controller.userSettings.pageSize}}
            @onPageChange={{fn @controller.handlePageChange "dhv"}}
            @onPageSizeChange={{@controller.setUserPageSize}}
          />
        {{else}}
          <div class="empty-message" data-test-empty-dhv-list-headline>
            {{#if @controller.dhvFilter}}
              <p>No dynamic host volumes match your search for "{{@controller.dhvFilter}}"</p>
              <HdsButton
                @text="Clear search"
                @color="secondary"
                {{on "click" (fn @controller.clearFilter "dhv")}}
              />
            {{else}}
              <p>No Dynamic Host Volumes found</p>
            {{/if}}
          </div>
        {{/if}}
      </HdsCardContainer>

      <HdsCardContainer
        @level="base"
        @hasBorder={{false}}
        class="info-panels storage-index-table-card"
      >
        <header aria-label="Other Storage Types">
          <h3>Other Storage Types</h3>
        </header>
        <HdsAlert @type="inline" @color="highlight" @icon="hard-drive" as |A|>
          <A.Title>
            Static Host Volumes
          </A.Title>
          <A.Description>
            Defined in the Nomad agent's config file, best for infrequently
            changing storage
          </A.Description>
          <A.Button
            @color="secondary"
            @icon="arrow-right"
            @iconPosition="trailing"
            @text="Learn more"
            @href="https://developer.hashicorp.com/nomad/docs/stateful-workloads/static-host-volumes"
          />
        </HdsAlert>
        <HdsAlert @type="inline" @color="highlight" @icon="hard-drive" as |A|>
          <A.Title>
            Ephemeral Disks
          </A.Title>
          <A.Description>
            Best-effort persistence, ideal for rebuildable data. Stored in the
            <code>/alloc/data</code>
            directory in a given allocation.
          </A.Description>
          <A.Button
            @color="secondary"
            @icon="arrow-right"
            @iconPosition="trailing"
            @text="Learn more"
            @href="https://developer.hashicorp.com/nomad/docs/architecture/storage/stateful-workloads#ephemeral-disks"
          />
        </HdsAlert>
      </HdsCardContainer>
    {{/if}}
  </section>
</template>
