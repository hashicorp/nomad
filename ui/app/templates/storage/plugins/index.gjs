/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import { pageTitle } from 'ember-page-title';
import { gt } from 'ember-truth-helpers';
import eq from 'ember-truth-helpers/helpers/eq';
import ForbiddenMessage from 'nomad-ui/components/forbidden-message';
import ListPagination from 'nomad-ui/components/list-pagination';
import ListTable from 'nomad-ui/components/list-table';
import PageSizeSelect from 'nomad-ui/components/page-size-select';
import SearchBox from 'nomad-ui/components/search-box';
import StorageSubnav from 'nomad-ui/components/storage-subnav';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';

<template>
  {{pageTitle "CSI Plugins"}}
  <StorageSubnav />

  <section class="section">
    {{#if @controller.isForbidden}}
      <ForbiddenMessage />
    {{else}}
      <div class="toolbar">
        <div class="toolbar-item">
          {{#if @model.length}}
            <SearchBox
              data-test-plugins-search
              @searchTerm={{@controller.searchTerm}}
              @onChange={{@controller.updateSearchTerm}}
              @placeholder="Search plugins..."
            />
          {{/if}}
        </div>
      </div>

      {{#if @controller.sortedPlugins}}
        <ListPagination
          @source={{@controller.sortedPlugins}}
          @size={{@controller.pageSize}}
          @page={{@controller.currentPage}}
          as |p|
        >
          <ListTable
            @source={{p.list}}
            @sortProperty={{@controller.sortProperty}}
            @sortDescending={{@controller.sortDescending}}
            @class="with-foot"
            as |t|
          >
            <t.head>
              <t.sortBy @prop="plainId">ID</t.sortBy>
              <t.sortBy @prop="controllersHealthyProportion">Controller Health</t.sortBy>
              <t.sortBy @prop="nodesHealthyProportion">Node Health</t.sortBy>
              <t.sortBy @prop="provider">Provider</t.sortBy>
            </t.head>
            <t.body @key="model.id" as |row|>
              <tr
                class="is-interactive"
                data-test-plugin-row
                {{on "click" (fn @controller.gotoPlugin row.model)}}
              >
                <td
                  data-test-plugin-id
                  {{keyboardShortcut
                    enumerated=true
                    action=(fn @controller.gotoPlugin row.model)
                  }}
                >
                  <LinkTo
                    @route="storage.plugins.plugin"
                    @model={{row.model.plainId}}
                    class="is-primary"
                  >{{row.model.plainId}}</LinkTo>
                </td>
                <td data-test-plugin-controller-health>
                  {{#if row.model.controllerRequired}}
                    {{if
                      (gt row.model.controllersHealthy 0)
                      "Healthy"
                      "Unhealthy"
                    }}
                    ({{row.model.controllersHealthy}}/{{row.model.controllersExpected}})
                  {{else}}
                    {{#if (gt row.model.controllersExpected 0)}}
                      {{if
                        (gt row.model.controllersHealthy 0)
                        "Healthy"
                        "Unhealthy"
                      }}
                      ({{row.model.controllersHealthy}}/{{row.model.controllersExpected}})
                    {{else}}
                      <em class="is-faded">Node Only</em>
                    {{/if}}
                  {{/if}}
                </td>
                <td data-test-plugin-node-health>
                  {{if (gt row.model.nodesHealthy 0) "Healthy" "Unhealthy"}}
                  ({{row.model.nodesHealthy}}/{{row.model.nodesExpected}})
                </td>
                <td data-test-plugin-provider>{{row.model.provider}}</td>
              </tr>
            </t.body>
          </ListTable>

          <div class="table-foot">
            <PageSizeSelect @onChange={{@controller.resetPagination}} />
            <nav class="pagination">
              <div class="pagination-numbers">
                {{p.startsAt}}&ndash;{{p.endsAt}}
                of
                {{@controller.sortedPlugins.length}}
              </div>
              <p.prev @class="pagination-previous">
                <HdsIcon @name="chevron-left" />
              </p.prev>
              <p.next @class="pagination-next">
                <HdsIcon @name="chevron-right" />
              </p.next>
              <ul class="pagination-list"></ul>
            </nav>
          </div>
        </ListPagination>
      {{else}}
        <div data-test-empty-plugins-list class="empty-message">
          {{#if (eq @model.length 0)}}
            <h3
              data-test-empty-plugins-list-headline
              class="empty-message-headline"
            >No Plugins</h3>
            <p class="empty-message-body">
              The cluster currently has no registered CSI Plugins.
            </p>
          {{else if @controller.searchTerm}}
            <h3
              data-test-empty-plugins-list-headline
              class="empty-message-headline"
            >No Matches</h3>
            <p class="empty-message-body">
              No plugins match the term
              <strong>{{@controller.searchTerm}}</strong>
            </p>
          {{/if}}
        </div>
      {{/if}}
    {{/if}}
  </section>
</template>
