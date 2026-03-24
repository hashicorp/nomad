/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { fn } from '@ember/helper';
import { pageTitle } from 'ember-page-title';
import eq from 'ember-truth-helpers/helpers/eq';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import ListPagination from 'nomad-ui/components/list-pagination';
import ListTable from 'nomad-ui/components/list-table';
import MultiSelectDropdown from 'nomad-ui/components/multi-select-dropdown';
import PageSizeSelect from 'nomad-ui/components/page-size-select';
import PluginAllocationRow from 'nomad-ui/components/plugin-allocation-row';
import PluginSubnav from 'nomad-ui/components/plugin-subnav';

<template>
  {{pageTitle "CSI Plugin " @model.plainId " allocations"}}
  <PluginSubnav @plugin={{@model}} />

  <section class="section">
    <div class="toolbar">
      <div class="toolbar-item">
        <h1 class="title">Allocations for {{@model.plainId}}</h1>
      </div>
      <div class="toolbar-item is-right-aligned is-mobile-full-width">
        <div class="button-bar">
          <MultiSelectDropdown
            data-test-health-facet
            @label="Health"
            @options={{@controller.optionsHealth}}
            @selection={{@controller.selectionHealth}}
            @onSelect={{fn @controller.setFacetQueryParam "qpHealth"}}
          />
          <MultiSelectDropdown
            data-test-type-facet
            @label="Type"
            @options={{@controller.optionsType}}
            @selection={{@controller.selectionType}}
            @onSelect={{fn @controller.setFacetQueryParam "qpType"}}
          />
        </div>
      </div>
    </div>

    {{#if @controller.sortedAllocations}}
      <ListPagination
        @source={{@controller.sortedAllocations}}
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
            <th class="is-narrow"><span class="visually-hidden">Driver Health,
                Scheduling, and Preemption</span></th>
            <td>ID</td>
            <th>Created</th>
            <t.sortBy @prop="updateTime">Modified</t.sortBy>
            <t.sortBy @prop="healthy">Health</t.sortBy>
            <th>Client</th>
            <th>Job</th>
            <th>Version</th>
            <th>Volumes</th>
            <th>CPU</th>
            <th>Memory</th>
          </t.head>
          <t.body @key="model.allocID" as |row|>
            <PluginAllocationRow
              data-test-allocation={{row.model.allocID}}
              @pluginAllocation={{row.model}}
            />
          </t.body>
        </ListTable>

        <div class="table-foot">
          <PageSizeSelect @onChange={{@controller.resetPagination}} />
          <nav class="pagination">
            <div class="pagination-numbers">
              {{p.startsAt}}&ndash;{{p.endsAt}}
              of
              {{@controller.sortedAllocations.length}}
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
      <div data-test-empty-list class="empty-message">
        {{#if (eq @controller.combinedAllocations.length 0)}}
          <h3 data-test-empty-list-headline class="empty-message-headline">No
            Allocations</h3>
          <p class="empty-message-body">
            The plugin has no allocations.
          </p>
        {{else if (eq @controller.sortedAllocations.length 0)}}
          <h3 data-test-empty-list-headline class="empty-message-headline">No
            Matches</h3>
          <p class="empty-message-body">
            No allocations match your current filter selection.
          </p>
        {{/if}}
      </div>
    {{/if}}
  </section>
</template>
