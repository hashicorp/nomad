/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { concat, fn } from '@ember/helper';
import { pageTitle } from 'ember-page-title';
import eq from 'ember-truth-helpers/helpers/eq';
import AllocationRow from 'nomad-ui/components/allocation-row';
import JobSubnav from 'nomad-ui/components/job-subnav';
import ListPagination from 'nomad-ui/components/list-pagination';
import ListTable from 'nomad-ui/components/list-table';
import MultiSelectDropdown from 'nomad-ui/components/multi-select-dropdown';
import SearchBox from 'nomad-ui/components/search-box';
import TaskSubRow from 'nomad-ui/components/task-sub-row';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';

<template>
  {{pageTitle "Job " @model.name " allocations"}}
  <JobSubnav @job={{@model}} />

  <section class="section">
    {{#if @controller.allocations.length}}
      <div class="toolbar">
        <div class="toolbar-item">
          <SearchBox
            data-test-allocations-search
            @searchTerm={{@controller.searchTerm}}
            @onChange={{@controller.updateSearchTerm}}
            @placeholder="Search allocations..."
          />
        </div>
        <div class="toolbar-item is-right-aligned">
          <div class="button-bar">
            <MultiSelectDropdown
              data-test-allocation-status-facet
              @label="Status"
              @options={{@controller.optionsAllocationStatus}}
              @selection={{@controller.selectionStatus}}
              @onSelect={{fn @controller.setFacetQueryParam "qpStatus"}}
            />
            <MultiSelectDropdown
              data-test-allocation-client-facet
              @label="Client"
              @options={{@controller.optionsClients}}
              @selection={{@controller.selectionClient}}
              @onSelect={{fn @controller.setFacetQueryParam "qpClient"}}
            />
            <MultiSelectDropdown
              data-test-allocation-task-group-facet
              @label="Task Group"
              @options={{@controller.optionsTaskGroups}}
              @selection={{@controller.selectionTaskGroup}}
              @onSelect={{fn @controller.setFacetQueryParam "qpTaskGroup"}}
            />
            <MultiSelectDropdown
              data-test-allocation-version-facet
              @label="Job Version"
              @options={{@controller.optionsVersions}}
              @selection={{@controller.selectionVersion}}
              @onSelect={{fn @controller.setFacetQueryParam "qpVersion"}}
            />
            <MultiSelectDropdown
              @label="Scheduling"
              @options={{@controller.optionsScheduling}}
              @selection={{@controller.selectionScheduling}}
              @onSelect={{fn @controller.setFacetQueryParam "qpScheduling"}}
            />
          </div>
        </div>
      </div>

      {{#if @controller.sortedAllocations}}
        <ListPagination
          @source={{@controller.sortedAllocations}}
          @size={{@controller.pageSize}}
          @page={{@controller.currentPage}}
          @class="allocations"
          as |p|
        >
          <ListTable
            @source={{p.list}}
            @sortProperty={{@controller.sortProperty}}
            @sortDescending={{@controller.sortDescending}}
            @class="with-foot with-collapsed-borders"
            as |t|
          >
            <t.head>
              <th class="is-narrow"><span class="visually-hidden">Driver Health,
                  Scheduling, and Preemption</span></th>
              <t.sortBy @prop="shortId">ID</t.sortBy>
              <t.sortBy @prop="taskGroupName">Task Group</t.sortBy>
              <t.sortBy
                @prop="createIndex"
                @title="Create Index"
              >Created</t.sortBy>
              <t.sortBy
                @prop="modifyIndex"
                @title="Modify Index"
              >Modified</t.sortBy>
              <t.sortBy @prop="statusIndex">Status</t.sortBy>
              <t.sortBy @prop="jobVersion">Version</t.sortBy>
              <t.sortBy @prop="node.shortId">Client</t.sortBy>
              <th>Volume</th>
              <th>CPU</th>
              <th>Memory</th>
              {{#if @controller.job.actions.length}}
                <th>Actions</th>
              {{/if}}
            </t.head>
            <t.body as |row|>
              <AllocationRow
                {{keyboardShortcut
                  enumerated=true
                  action=(fn @controller.gotoAllocation row.model)
                }}
                data-test-allocation={{row.model.id}}
                @allocation={{row.model}}
                @context="job"
                @onClick={{fn @controller.gotoAllocation row.model}}
              />
              {{#each row.model.states as |task|}}
                <TaskSubRow
                  @active={{eq
                    @controller.activeTask
                    (concat task.allocation.id "-" task.name)
                  }}
                  @onSetActiveTask={{@controller.setActiveTaskQueryParam}}
                  @namespan="9"
                  @taskState={{task}}
                  @jobHasActions={{@controller.job.actions.length}}
                />
              {{/each}}

            </t.body>
          </ListTable>

          <div class="table-foot">
            <nav class="pagination">
              <div class="pagination-numbers">
                {{p.startsAt}}&ndash;{{p.endsAt}}
                of
                {{@controller.sortedAllocations.length}}
              </div>
              <p.prev @class="pagination-previous"> &lt; </p.prev>
              <p.next @class="pagination-next"> &gt; </p.next>
              <ul class="pagination-list"></ul>
            </nav>
          </div>
        </ListPagination>
      {{else}}
        <div class="boxed-section-body">
          <div class="empty-message" data-test-empty-allocations-list>
            <h3
              class="empty-message-headline"
              data-test-empty-allocations-list-headline
            >No Matches</h3>
            <p class="empty-message-body">No allocations match the term
              <strong>{{@controller.searchTerm}}</strong></p>
          </div>
        </div>
      {{/if}}
    {{else}}
      <div class="boxed-section-body">
        <div class="empty-message" data-test-empty-allocations-list>
          <h3
            class="empty-message-headline"
            data-test-empty-allocations-list-headline
          >No Allocations</h3>
          <p class="empty-message-body">No allocations have been placed.</p>
        </div>
      </div>
    {{/if}}
  </section>
</template>
