/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, concat, fn, hash } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import { pageTitle } from 'ember-page-title';
import can from 'ember-can/helpers/can';
import cannot from 'ember-can/helpers/cannot';
import and from 'ember-truth-helpers/helpers/and';
import eq from 'ember-truth-helpers/helpers/eq';
import gt from 'ember-truth-helpers/helpers/gt';
import or from 'ember-truth-helpers/helpers/or';
import AllocationRow from 'nomad-ui/components/allocation-row';
import AllocationStatusBar from 'nomad-ui/components/allocation-status-bar';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import ExecOpenButton from 'nomad-ui/components/exec/open-button';
import JobPagePartsMeta from 'nomad-ui/components/job-page/parts/meta';
import JobPagePartsSummaryLegendItem from 'nomad-ui/components/job-page/parts/summary-legend-item';
import LifecycleChart from 'nomad-ui/components/lifecycle-chart';
import ListPagination from 'nomad-ui/components/list-pagination';
import ListTable from 'nomad-ui/components/list-table';
import MultiSelectDropdown from 'nomad-ui/components/multi-select-dropdown';
import PageSizeSelect from 'nomad-ui/components/page-size-select';
import ScaleEventsAccordion from 'nomad-ui/components/scale-events-accordion';
import ScaleEventsChart from 'nomad-ui/components/scale-events-chart';
import SearchBox from 'nomad-ui/components/search-box';
import StepperInput from 'nomad-ui/components/stepper-input';
import TaskSubRow from 'nomad-ui/components/task-sub-row';
import Toggle from 'nomad-ui/components/toggle';
import formatScheduledBytes from 'nomad-ui/helpers/format-scheduled-bytes';
import formatScheduledHertz from 'nomad-ui/helpers/format-scheduled-hertz';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';

<template>
  <Breadcrumb @crumb={{@controller.breadcrumb}} />
  {{pageTitle "Task group " @model.name " - Job " @model.job.name}}

  <div class="tabs is-subnav">
    <ul>
      <li>
        <LinkTo
          @route="jobs.job.task-group"
          @models={{array @model.job @model}}
          @activeClass="is-active"
        >
          Overview
        </LinkTo>
      </li>
    </ul>
  </div>

  <section class="section">
    <h1 class="title with-flex">
      <span>
        {{@model.name}}
      </span>
      <div>
        <ExecOpenButton @job={{@model.job}} @taskGroup={{@model}} />
        {{#if @model.scaling}}
          <StepperInput
            data-test-task-group-count-stepper
            aria-label={{@controller.tooltipText}}
            @min={{@model.scaling.min}}
            @max={{@model.scaling.max}}
            @value={{@model.count}}
            @class="is-primary is-small"
            @disabled={{or
              @model.job.runningDeployment
              (cannot "scale job" namespace=@model.job.namespace.name)
            }}
            @onChange={{@controller.scaleTaskGroup}}
          >
            Count
          </StepperInput>
        {{/if}}
      </div>
    </h1>

    <div class="boxed-section is-small">
      <div class="boxed-section-body inline-definitions">
        <span class="label">
          Task Group Details
        </span>
        <span class="pair" data-test-task-group-tasks>
          <span class="term">
            # Tasks
          </span>
          {{@model.tasks.length}}
        </span>
        <span class="pair" data-test-task-group-cpu>
          <span class="term">
            Reserved CPU
          </span>
          {{formatScheduledHertz @model.reservedCPU}}
        </span>
        <span class="pair" data-test-task-group-mem>
          <span class="term">
            Reserved Memory
          </span>
          {{formatScheduledBytes @model.reservedMemory start="MiB"}}
          {{#if (gt @model.reservedMemoryMax @model.reservedMemory)}}
            ({{formatScheduledBytes @model.reservedMemoryMax start="MiB"}}Max)
          {{/if}}
        </span>
        <span class="pair" data-test-task-group-disk>
          <span class="term">
            Reserved Disk
          </span>
          {{formatScheduledBytes @model.reservedEphemeralDisk start="MiB"}}
        </span>
        <span class="pair">
          <span class="term">
            Namespace
          </span>
          {{@model.job.namespace.name}}
        </span>
        {{#if @model.scaling}}
          <span class="pair" data-test-task-group-min>
            <span class="term">
              Count Range
            </span>
            {{@model.scaling.min}}
            to
            {{@model.scaling.max}}
          </span>
          <span class="pair" data-test-task-group-max>
            <span class="term">
              Scaling Policy?
            </span>
            {{if @model.scaling.policy "Yes" "No"}}
          </span>
        {{/if}}
        {{#if (and (can "list variables") @model.pathLinkedVariable)}}
          <span class="pair" data-test-task-group-stat="variables">
            <LinkTo
              @route="variables.variable"
              @model={{@model.pathLinkedVariable.id}}
            >Variables</LinkTo>
          </span>
        {{/if}}
      </div>
    </div>

    <div class="boxed-section">
      <div class="boxed-section-head">
        <div>
          Allocation Status
          <span class="badge is-white">
            {{@controller.allocations.length}}
          </span>
        </div>
      </div>
      <div class="boxed-section-body">
        <AllocationStatusBar
          @allocationContainer={{@model.summary}}
          @class="split-view"
          as |chart|
        >
          <ol class="legend">
            {{#each chart.data as |datum index|}}
              <li
                class="{{datum.className}}
                  {{if (eq datum.label chart.activeDatum.label) 'is-active'}}
                  {{if (eq datum.value 0) 'is-empty'}}"
              >
                <JobPagePartsSummaryLegendItem
                  @datum={{datum}}
                  @index={{index}}
                />
              </li>
            {{/each}}
          </ol>
        </AllocationStatusBar>
      </div>
    </div>

    <div class="boxed-section">
      <div class="boxed-section-head">
        Allocations
        <div class="pull-right is-subsection">
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
          <SearchBox
            @searchTerm={{@controller.searchTerm}}
            @placeholder="Search allocations..."
            @onChange={{@controller.updateSearchTerm}}
            @class="is-padded"
            @inputClass="is-compact"
          />
          <span class="is-padded is-one-line">
            <Toggle
              @isActive={{@controller.showSubTasks}}
              @onToggle={{@controller.toggleShowSubTasks}}
              title="Show tasks of allocations"
            >
              Show Tasks
            </Toggle>
          </span>
        </div>
      </div>
      <div class="boxed-section-body is-full-bleed">
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
              @class="with-foot {{if
                @controller.showSubTasks
                'with-collapsed-borders'
              }}"
              as |t|
            >
              <t.head>
                <th class="is-narrow"><span class="visually-hidden">Driver
                    Health, Scheduling, and Preemption</span></th>
                <t.sortBy @prop="shortId">
                  ID
                </t.sortBy>
                <t.sortBy @prop="createIndex" @title="Create Index">
                  Created
                </t.sortBy>
                <t.sortBy @prop="modifyIndex" @title="Modify Index">
                  Modified
                </t.sortBy>
                <t.sortBy @prop="statusIndex">
                  Status
                </t.sortBy>
                <t.sortBy @prop="jobVersion">
                  Version
                </t.sortBy>
                <t.sortBy @prop="node.shortId">
                  Client
                </t.sortBy>
                <th>
                  Volume
                </th>
                <th>
                  CPU
                </th>
                <th>
                  Memory
                </th>
                {{#if @model.job.actions.length}}
                  <th>Actions</th>
                {{/if}}
              </t.head>
              <t.body @key="model.id" as |row|>
                <AllocationRow
                  {{keyboardShortcut
                    enumerated=true
                    action=(fn @controller.gotoAllocation row.model)
                  }}
                  data-test-allocation={{row.model.id}}
                  @allocation={{row.model}}
                  @context="taskGroup"
                  @onClick={{fn @controller.gotoAllocation row.model}}
                />
                {{#if @controller.showSubTasks}}
                  {{#each row.model.states as |task|}}
                    <TaskSubRow
                      @namespan="8"
                      @taskState={{task}}
                      @active={{eq
                        @controller.activeTask
                        (concat task.allocation.id "-" task.name)
                      }}
                      @onSetActiveTask={{@controller.setActiveTaskQueryParam}}
                      @jobHasActions={{@model.job.actions.length}}
                    />
                  {{/each}}
                {{/if}}
              </t.body>
            </ListTable>
            <div class="table-foot">
              <PageSizeSelect @onChange={{@controller.resetPagination}} />
              <nav class="pagination">
                <div class="pagination-numbers">
                  {{p.startsAt}}
                  &ndash;
                  {{p.endsAt}}
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
        {{else if @controller.allocations.length}}
          <div class="boxed-section-body">
            <div class="empty-message" data-test-empty-allocations-list>
              <h3
                class="empty-message-headline"
                data-test-empty-allocations-list-headline
              >
                No Matches
              </h3>
              <p class="empty-message-body">
                No allocations match the term
                <strong>
                  {{@controller.searchTerm}}
                </strong>
              </p>
            </div>
          </div>
        {{else}}
          <div class="boxed-section-body">
            <div class="empty-message" data-test-empty-allocations-list>
              <h3
                class="empty-message-headline"
                data-test-empty-allocations-list-headline
              >
                No Allocations
              </h3>
              <p class="empty-message-body">
                No allocations have been placed.
              </p>
            </div>
          </div>
        {{/if}}
      </div>
    </div>

    <LifecycleChart @tasks={{@model.tasks}} />

    {{#if @model.scaleState.isVisible}}
      {{#if @controller.shouldShowScaleEventTimeline}}
        <div data-test-scaling-timeline class="boxed-section">
          <div class="boxed-section-head is-hollow">
            Scaling Timeline
          </div>
          <div class="boxed-section-body">
            <ScaleEventsChart @events={{@controller.sortedScaleEvents}} />
          </div>
        </div>
      {{/if}}
      <div class="boxed-section">
        <div class="boxed-section-head">
          Recent Scaling Events
        </div>
        <div class="boxed-section-body">
          <ScaleEventsAccordion @events={{@controller.sortedScaleEvents}} />
        </div>
      </div>
    {{/if}}

    {{#if @model.volumes.length}}
      <div data-test-volumes class="boxed-section">
        <div class="boxed-section-head">
          Volume Requirements
        </div>
        <div class="boxed-section-body is-full-bleed">
          <ListTable @source={{@model.volumes}} as |t|>
            <t.head>
              <th>
                Name
              </th>
              <th>
                Type
              </th>
              <th>
                Source
              </th>
              <th>
                Permissions
              </th>
            </t.head>
            <t.body as |row|>
              <tr data-test-volume>
                <td data-test-volume-name>
                  {{#if row.model.isCSI}}
                    {{#if row.model.perAlloc}}
                      <LinkTo
                        @route="storage.volumes.index"
                        @query={{hash search=row.model.source}}
                      >{{row.model.name}}</LinkTo>
                    {{else}}
                      <LinkTo
                        @route="storage.volumes.volume"
                        @model={{concat
                          row.model.source
                          "@"
                          row.model.namespace.id
                        }}
                      >
                        {{row.model.name}}
                      </LinkTo>
                    {{/if}}
                  {{else}}
                    {{row.model.name}}
                  {{/if}}
                </td>
                <td data-test-volume-type>
                  {{row.model.type}}
                </td>
                <td data-test-volume-source>
                  {{row.model.source}}
                </td>
                <td data-test-volume-permissions>
                  {{if row.model.readOnly "Read" "Read/Write"}}
                </td>
              </tr>
            </t.body>
          </ListTable>
        </div>
      </div>
    {{/if}}

    {{#if @model.meta}}
      <JobPagePartsMeta @meta={{@model.meta}} />
    {{/if}}
  </section>
</template>
