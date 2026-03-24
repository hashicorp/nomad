/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { fn } from '@ember/helper';
import { pageTitle } from 'ember-page-title';
import JobClientStatusRow from 'nomad-ui/components/job-client-status-row';
import JobSubnav from 'nomad-ui/components/job-subnav';
import ListPagination from 'nomad-ui/components/list-pagination';
import ListTable from 'nomad-ui/components/list-table';
import MultiSelectDropdown from 'nomad-ui/components/multi-select-dropdown';
import SearchBox from 'nomad-ui/components/search-box';

<template>
  {{pageTitle "Job " @model.name " clients"}}
  <JobSubnav @job={{@model}} />

  <section class="section">
    {{#if @controller.nodes.length}}
      <div class="toolbar">
        <div class="toolbar-item">
          <SearchBox
            data-test-clients-search
            @searchTerm={{@controller.searchTerm}}
            @onChange={{@controller.updateSearchTerm}}
            @placeholder="Search clients..."
          />
        </div>
        <div class="toolbar-item is-right-aligned">
          <div class="button-bar">
            <MultiSelectDropdown
              data-test-job-status-facet
              @label="Job Status"
              @options={{@controller.optionsJobStatus}}
              @selection={{@controller.selectionStatus}}
              @onSelect={{fn @controller.setFacetQueryParam "qpStatus"}}
            />
            <MultiSelectDropdown
              data-test-datacenter-facet
              @label="Datacenter"
              @options={{@controller.optionsDatacenter}}
              @selection={{@controller.selectionDatacenter}}
              @onSelect={{fn @controller.setFacetQueryParam "qpDatacenter"}}
            />
            <MultiSelectDropdown
              data-test-client-class-facet
              @label="Client Class"
              @options={{@controller.optionsClientClass}}
              @selection={{@controller.selectionClientClass}}
              @onSelect={{fn @controller.setFacetQueryParam "qpClientClass"}}
            />
          </div>
        </div>
      </div>

      {{#if @controller.sortedClients}}
        <ListPagination
          @source={{@controller.sortedClients}}
          @size={{@controller.pageSize}}
          @page={{@controller.currentPage}}
          @class="clients"
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
              <t.sortBy @prop="node.id">Client ID</t.sortBy>
              <t.sortBy @prop="node.name" class="is-200px is-truncatable">Client
                Name</t.sortBy>
              <t.sortBy
                @prop="createTime"
                @title="Create Time"
              >Created</t.sortBy>
              <t.sortBy
                @prop="modifyTime"
                @title="Modify Time"
              >Modified</t.sortBy>
              <t.sortBy @prop="jobStatus">Job Status</t.sortBy>
              <th class="is-3">Allocation Summary</th>
            </t.head>
            <t.body as |row|>
              <JobClientStatusRow
                @row={{row}}
                @onClick={{@controller.gotoClient}}
              />
            </t.body>
          </ListTable>

          <div class="table-foot">
            <nav class="pagination">
              <div class="pagination-numbers">
                {{p.startsAt}}&ndash;{{p.endsAt}}
                of
                {{@controller.sortedClients.length}}
              </div>
              <p.prev @class="pagination-previous"> &lt; </p.prev>
              <p.next @class="pagination-next"> &gt; </p.next>
              <ul class="pagination-list"></ul>
            </nav>
          </div>
        </ListPagination>
      {{else}}
        <div class="boxed-section-body">
          <div class="empty-message" data-test-empty-clients-list>
            <h3
              class="empty-message-headline"
              data-test-empty-clients-list-headline
            >
              No Matches
            </h3>
            <p class="empty-message-body">
              No clients match the term
              <strong>
                {{@controller.searchTerm}}
              </strong>
            </p>
          </div>
        </div>
      {{/if}}
    {{else}}
      <div class="boxed-section-body">
        <div class="empty-message" data-test-empty-clients-list>
          <h3
            class="empty-message-headline"
            data-test-empty-clients-list-headline
          >
            No Clients
          </h3>
          <p class="empty-message-body">
            No clients available.
          </p>
        </div>
      </div>
    {{/if}}
  </section>
</template>
