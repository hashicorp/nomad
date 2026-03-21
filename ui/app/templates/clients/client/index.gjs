/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, concat, fn, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { pageTitle } from 'ember-page-title';
import can from 'ember-can/helpers/can';
import cannot from 'ember-can/helpers/cannot';
import perform from 'ember-concurrency/helpers/perform';
import eq from 'ember-truth-helpers/helpers/eq';
import not from 'ember-truth-helpers/helpers/not';
import or from 'ember-truth-helpers/helpers/or';
import momentFromNow from 'ember-moment/helpers/moment-from-now';
import momentToNow from 'ember-moment/helpers/moment-to-now';
import AllocationRow from 'nomad-ui/components/allocation-row';
import AttributesTable from 'nomad-ui/components/attributes-table';
import ClientSubnav from 'nomad-ui/components/client-subnav';
import CopyButton from 'nomad-ui/components/copy-button';
import DrainPopover from 'nomad-ui/components/drain-popover';
import ListAccordion from 'nomad-ui/components/list-accordion';
import ListPagination from 'nomad-ui/components/list-pagination';
import ListTable from 'nomad-ui/components/list-table';
import MetadataEditor from 'nomad-ui/components/metadata-editor';
import MultiSelectDropdown from 'nomad-ui/components/multi-select-dropdown';
import PrimaryMetricNode from 'nomad-ui/components/primary-metric/node';
import SearchBox from 'nomad-ui/components/search-box';
import TaskSubRow from 'nomad-ui/components/task-sub-row';
import Toggle from 'nomad-ui/components/toggle';
import TwoStepButton from 'nomad-ui/components/two-step-button';
import capitalize from 'nomad-ui/helpers/capitalize';
import formatDuration from 'nomad-ui/helpers/format-duration';
import formatTs from 'nomad-ui/helpers/format-ts';
import pluralize from 'nomad-ui/helpers/pluralize';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import {
  HdsIcon,
  HdsTooltipButton,
} from '@hashicorp/design-system-components/components';

<template>
  {{pageTitle "Client " (or @model.name @model.shortId)}}
  <ClientSubnav @client={{@model}} />
  <section class="section">
    {{#if @controller.eligibilityError}}
      <div data-test-eligibility-error class="columns">
        <div class="column">
          <div class="notification is-danger">
            <h3 data-test-title class="title is-4">
              Eligibility Error
            </h3>
            <p data-test-message>
              {{@controller.eligibilityError}}
            </p>
          </div>
        </div>
        <div class="column is-centered is-minimum">
          <button
            data-test-dismiss
            class="button is-danger"
            {{on "click" @controller.clearEligibilityError}}
            type="button"
          >
            Okay
          </button>
        </div>
      </div>
    {{/if}}
    {{#if @controller.stopDrainError}}
      <div data-test-stop-drain-error class="columns">
        <div class="column">
          <div class="notification is-danger">
            <h3 data-test-title class="title is-4">
              Stop Drain Error
            </h3>
            <p data-test-message>
              {{@controller.stopDrainError}}
            </p>
          </div>
        </div>
        <div class="column is-centered is-minimum">
          <button
            data-test-dismiss
            class="button is-danger"
            {{on "click" @controller.clearStopDrainError}}
            type="button"
          >
            Okay
          </button>
        </div>
      </div>
    {{/if}}
    {{#if @controller.drainError}}
      <div data-test-drain-error class="columns">
        <div class="column">
          <div class="notification is-danger">
            <h3 data-test-title class="title is-4">
              Drain Error
            </h3>
            <p data-test-message>
              {{@controller.drainError}}
            </p>
          </div>
        </div>
        <div class="column is-centered is-minimum">
          <button
            data-test-dismiss
            class="button is-danger"
            {{on "click" @controller.clearDrainError}}
            type="button"
          >
            Okay
          </button>
        </div>
      </div>
    {{/if}}
    {{#if @controller.showDrainStoppedNotification}}
      <div class="notification is-info">
        <div data-test-drain-stopped-notification class="columns">
          <div class="column">
            <h3 data-test-title class="title is-4">
              Drain Stopped
            </h3>
            <p data-test-message>
              The drain has been stopped and the node has been set to
              ineligible.
            </p>
          </div>
          <div class="column is-centered is-minimum">
            <button
              data-test-dismiss
              class="button is-info"
              {{on "click" @controller.dismissDrainStoppedNotification}}
              type="button"
            >
              Okay
            </button>
          </div>
        </div>
      </div>
    {{/if}}
    {{#if @controller.showDrainUpdateNotification}}
      <div class="notification is-info">
        <div data-test-drain-updated-notification class="columns">
          <div class="column">
            <h3 data-test-title class="title is-4">
              Drain Updated
            </h3>
            <p data-test-message>
              The new drain specification has been applied.
            </p>
          </div>
          <div class="column is-centered is-minimum">
            <button
              data-test-dismiss
              class="button is-info"
              {{on "click" @controller.dismissDrainUpdateNotification}}
              type="button"
            >
              Okay
            </button>
          </div>
        </div>
      </div>
    {{/if}}
    {{#if @controller.showDrainNotification}}
      <div class="notification is-info">
        <div data-test-drain-complete-notification class="columns">
          <div class="column">
            <h3 data-test-title class="title is-4">
              Drain Complete
            </h3>
            <p data-test-message>
              Allocations have been drained and the node has been set to
              ineligible.
            </p>
          </div>
          <div class="column is-centered is-minimum">
            <button
              data-test-dimiss
              class="button is-info"
              {{on "click" @controller.dismissDrainNotification}}
              type="button"
            >
              Okay
            </button>
          </div>
        </div>
      </div>
    {{/if}}
    <div class="toolbar">
      <div class="toolbar-item is-top-aligned is-minimum">
        <span class="title">
          <span
            data-test-node-status="{{@model.compositeStatus}}"
            class="node-status-light {{@model.compositeStatus}}"
          >
            <HdsIcon @name={{@model.compositeStatusIcon}} @isInline={{true}} />
          </span>
        </span>
      </div>
      <div class="toolbar-item">
        <h1 data-test-title class="title with-subheading">
          {{or @model.name @model.shortId}}
        </h1>
        <p>
          <label class="is-interactive">
            <Toggle
              data-test-eligibility-toggle
              @isActive={{@model.isEligible}}
              @isDisabled={{or
                @controller.setEligibility.isRunning
                @model.isDraining
                (cannot "write client")
              }}
              @onToggle={{perform
                @controller.setEligibility
                (not @model.isEligible)
              }}
            >
              Eligible
            </Toggle>
          </label>
          <HdsTooltipButton
            @text="Only eligible clients can receive allocations"
            aria-label="More information"
            class="is-faded"
          >
            <HdsIcon @name="info" @isInline={{true}} />
          </HdsTooltipButton>
          <span
            data-test-node-id
            class="tag is-hollow is-small no-text-transform"
          >
            {{@model.id}}
            <CopyButton
              @clipboardText={{@model.id}}
              @compact={{true}}
              @inset={{true}}
            />
          </span>
        </p>
      </div>
      <div class="toolbar-item is-right-aligned is-top-aligned">
        {{#if @model.isDraining}}
          <TwoStepButton
            data-test-drain-stop
            @idleText="Stop Drain"
            @cancelText="Cancel"
            @confirmText="Yes, Stop Drain"
            @confirmationMessage="Are you sure you want to stop this drain?"
            @awaitingConfirmation={{@controller.stopDrain.isRunning}}
            @onConfirm={{perform @controller.stopDrain}}
          />
        {{/if}}
      </div>
      <div class="toolbar-item is-right-aligned is-top-aligned">
        <DrainPopover
          @client={{@model}}
          @isDisabled={{cannot "write client"}}
          @onDrain={{@controller.drainNotify}}
          @onError={{@controller.setDrainError}}
        />
      </div>
    </div>
    <div class="boxed-section is-small">
      <div class="boxed-section-body inline-definitions">
        <span class="label">
          Client Details
        </span>
        <span class="pair" data-test-status-definition>
          <span class="term">
            Status
          </span>
          <span class="status-text node-{{@model.status}}">
            {{@model.status}}
          </span>
        </span>
        <span class="pair" data-test-address-definition>
          <span class="term">
            Address
          </span>
          {{@model.httpAddr}}
        </span>
        <span class="pair" data-test-datacenter-definition>
          <span class="term">
            Datacenter
          </span>
          {{@model.datacenter}}
        </span>
        <span class="pair" data-test-node-pool>
          <span class="term">
            Node Pool
          </span>
          {{#if @model.nodePool}}{{@model.nodePool}}{{else}}-{{/if}}
        </span>
        {{#if @model.nodeClass}}
          <span class="pair" data-test-node-class>
            <span class="term">
              Class
            </span>
            {{@model.nodeClass}}
          </span>
        {{/if}}
        <span class="pair" data-test-driver-health>
          <span class="term">
            Drivers
          </span>
          {{#if @model.unhealthyDrivers.length}}
            <HdsIcon
              @name="alert-triangle-fill"
              @color="warning"
              @isInline={{true}}
            />
            {{@model.unhealthyDrivers.length}}
            of
            {{@model.detectedDrivers.length}}
            {{pluralize "driver" @model.detectedDrivers.length}}
            unhealthy
          {{else}}
            All healthy
          {{/if}}
        </span>
      </div>
    </div>
    {{#if @model.drainStrategy}}
      <div data-test-drain-details class="boxed-section is-info">
        <div class="boxed-section-head">
          <div class="boxed-section-row">
            Drain Strategy
          </div>
          <div class="boxed-section-row">
            <div class="inline-definitions is-small">
              {{#unless @model.drainStrategy.hasNoDeadline}}
                <span class="pair">
                  <span class="term">
                    Duration
                  </span>
                  {{#if @model.drainStrategy.isForced}}
                    <span data-test-duration>
                      --
                    </span>
                  {{else}}
                    <span
                      data-test-duration
                      class="tooltip"
                      aria-label={{formatDuration
                        @model.drainStrategy.deadline
                      }}
                    >
                      {{formatDuration @model.drainStrategy.deadline}}
                    </span>
                  {{/if}}
                </span>
              {{/unless}}
              <span class="pair">
                <span class="term">
                  {{if
                    @model.drainStrategy.hasNoDeadline
                    "Deadline"
                    "Remaining"
                  }}
                </span>
                {{#if @model.drainStrategy.hasNoDeadline}}
                  <span data-test-deadline>
                    No deadline
                  </span>
                {{else if @model.drainStrategy.isForced}}
                  <span data-test-deadline>
                    --
                  </span>
                {{else}}
                  <span
                    data-test-deadline
                    class="tooltip"
                    aria-label={{formatTs @model.drainStrategy.forceDeadline}}
                  >
                    {{momentFromNow
                      @model.drainStrategy.forceDeadline
                      interval=1000
                      hideAffix=true
                    }}
                  </span>
                {{/if}}
              </span>
              <span data-test-force-drain-text class="pair">
                <span class="term">
                  Force Drain
                </span>
                {{#if @model.drainStrategy.isForced}}
                  <HdsIcon
                    @name="alert-triangle-fill"
                    @color="warning"
                    @isInline={{true}}
                  />Yes
                {{else}}
                  No
                {{/if}}
              </span>
              <span data-test-drain-system-jobs-text class="pair">
                <span class="term">
                  Drain System Jobs
                </span>
                {{if @model.drainStrategy.ignoreSystemJobs "No" "Yes"}}
              </span>
            </div>
            {{#unless @model.drainStrategy.isForced}}
              <div class="pull-right">
                <TwoStepButton
                  data-test-force
                  @alignRight={{true}}
                  @classes={{hash
                    idleButton="is-warning"
                    confirmationMessage="inherit-color"
                    cancelButton="is-danger is-important"
                    confirmButton="is-warning"
                  }}
                  @idleText="Force Drain"
                  @cancelText="Cancel"
                  @confirmText="Yes, Force Drain"
                  @confirmationMessage="Are you sure you want to force drain?"
                  @awaitingConfirmation={{@controller.forceDrain.isRunning}}
                  @onConfirm={{perform @controller.forceDrain}}
                />
              </div>
            {{/unless}}
          </div>
        </div>
        <div class="boxed-section-body">
          <div class="columns">
            <div class="column nowrap is-minimum">
              <div class="metric-group">
                <div class="metric is-primary">
                  <h3 class="label">
                    Complete
                  </h3>
                  <p data-test-complete-count class="value">
                    {{@model.completeAllocations.length}}
                  </p>
                </div>
              </div>
              <div class="metric-group">
                <div class="metric">
                  <h3 class="label">
                    Migrating
                  </h3>
                  <p data-test-migrating-count class="value">
                    {{@model.migratingAllocations.length}}
                  </p>
                </div>
              </div>
              <div class="metric-group">
                <div class="metric">
                  <h3 class="label">
                    Remaining
                  </h3>
                  <p data-test-remaining-count class="value">
                    {{@model.runningAllocations.length}}
                  </p>
                </div>
              </div>
            </div>
            <div class="column">
              <h3 class="title is-4">
                Status
              </h3>
              {{#if @model.lastMigrateTime}}
                <p data-test-status>
                  {{momentToNow
                    @model.lastMigrateTime
                    interval=1000
                    hideAffix=true
                  }}
                  since an allocation was successfully migrated.
                </p>
              {{else}}
                <p data-test-status>
                  No allocations migrated.
                </p>
              {{/if}}
            </div>
          </div>
        </div>
      </div>
    {{/if}}
    <div class="boxed-section">
      <div class="boxed-section-head is-hollow">
        Host Resource Utilization
        <HdsTooltipButton
          @text="All allocation and system processes aggregated"
          aria-label="More information"
          class="is-faded"
        >
          <HdsIcon @name="info" @isInline={{true}} />
        </HdsTooltipButton>

      </div>
      <div class="boxed-section-body">
        <div class="columns">
          <div class="column">
            <PrimaryMetricNode @node={{@model}} @metric="cpu" />
          </div>
          <div class="column">
            <PrimaryMetricNode @node={{@model}} @metric="memory" />
          </div>
        </div>
      </div>
    </div>
    <div class="boxed-section">
      <div class="boxed-section-head">
        <div>
          Allocations
          <button
            class="badge is-white"
            {{on "click" (fn @controller.setPreemptionFilter false)}}
            data-test-filter-all
            type="button"
          >
            {{@model.allocations.length}}
          </button>
          {{#if @controller.preemptions.length}}
            <button
              class="badge is-warning"
              {{on "click" (fn @controller.setPreemptionFilter true)}}
              data-test-filter-preemptions
              type="button"
            >
              {{@controller.preemptions.length}}
              {{pluralize "preemption" @controller.preemptions.length}}
            </button>
          {{/if}}
        </div>
        <div class="pull-right is-subsection">
          <MultiSelectDropdown
            data-test-allocation-namespace-facet
            @label="Namespace"
            @options={{@controller.optionsNamespace}}
            @selection={{@controller.selectionNamespace}}
            @onSelect={{fn @controller.setFacetQueryParam "qpNamespace"}}
          />
          <MultiSelectDropdown
            data-test-allocation-job-facet
            @label="Job"
            @options={{@controller.optionsJob}}
            @selection={{@controller.selectionJob}}
            @onSelect={{fn @controller.setFacetQueryParam "qpJob"}}
          />
          <MultiSelectDropdown
            data-test-allocation-status-facet
            @label="Status"
            @options={{@controller.optionsAllocationStatus}}
            @selection={{@controller.selectionStatus}}
            @onSelect={{fn @controller.setFacetQueryParam "qpStatus"}}
          />
          <SearchBox
            @searchTerm={{@controller.searchTerm}}
            @onChange={{@controller.updateSearchTerm}}
            @placeholder="Search allocations..."
            @inputClass="is-compact"
            @class="is-padded"
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
      <div
        class="boxed-section-body
          {{if @controller.sortedAllocations.length 'is-full-bleed'}}"
      >
        {{#if @controller.sortedAllocations.length}}
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
                <t.sortBy @prop="job.name">
                  Job
                </t.sortBy>
                <t.sortBy @prop="jobVersion">
                  Version
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
                <th>Actions</th>
              </t.head>
              <t.body as |row|>
                <AllocationRow
                  {{keyboardShortcut
                    enumerated=true
                    action=(fn @controller.gotoAllocation row.model)
                  }}
                  @allocation={{row.model}}
                  @context="node"
                  @onClick={{fn @controller.gotoAllocation row.model}}
                  data-test-allocation={{row.model.id}}
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
                      @jobHasActions={{true}}
                    />
                  {{/each}}
                {{/if}}
              </t.body>
            </ListTable>
            <div class="table-foot">
              <nav class="pagination">
                <div class="pagination-numbers">
                  {{p.startsAt}}
                  –
                  {{p.endsAt}}
                  of
                  {{@controller.sortedAllocations.length}}
                </div>
                <p.prev @class="pagination-previous">
                  &lt;
                </p.prev>
                <p.next @class="pagination-next">
                  >
                </p.next>
                <ul class="pagination-list"></ul>
              </nav>
            </div>
          </ListPagination>
        {{else}}
          <div data-test-empty-allocations-list class="empty-message">
            {{#if (eq @controller.visibleAllocations.length 0)}}
              <h3
                data-test-empty-allocations-list-headline
                class="empty-message-headline"
              >
                No Allocations
              </h3>
              <p
                data-test-empty-allocations-list-body
                class="empty-message-body"
              >
                The node doesn't have any allocations.
              </p>
            {{else if @controller.searchTerm}}
              <h3
                data-test-empty-allocations-list-headline
                class="empty-message-headline"
              >
                No Matches
              </h3>
              <p class="empty-message-body">
                No allocations match the term
                <strong>
                  {{@controller.searchTerm}}
                </strong>
              </p>
            {{else if (eq @controller.sortedAllocations.length 0)}}
              <h3
                data-test-empty-allocations-list-headline
                class="empty-message-headline"
              >
                No Matches
              </h3>
              <p class="empty-message-body">
                No allocations match your current filter selection.
              </p>
            {{/if}}
          </div>
        {{/if}}
      </div>
    </div>
    <div data-test-client-events class="boxed-section">
      <div class="boxed-section-head">
        Client Events
      </div>
      <div class="boxed-section-body is-full-bleed">
        <ListTable
          @source={{@controller.sortedEvents}}
          @class="is-striped"
          as |t|
        >
          <t.head>
            <th class="is-2">
              Time
            </th>
            <th class="is-2">
              Subsystem
            </th>
            <th>
              Message
            </th>
          </t.head>
          <t.body as |row|>
            <tr data-test-client-event>
              <td data-test-client-event-time>
                {{formatTs row.model.time}}
              </td>
              <td data-test-client-event-subsystem>
                {{row.model.subsystem}}
              </td>
              <td data-test-client-event-message>
                {{#if row.model.message}}
                  {{#if row.model.driver}}
                    <span class="badge is-secondary is-small">
                      {{row.model.driver}}
                    </span>
                  {{/if}}
                  {{row.model.message}}
                {{else}}
                  <em>
                    No message
                  </em>
                {{/if}}
              </td>
            </tr>
          </t.body>
        </ListTable>
      </div>
    </div>
    {{#if @controller.sortedHostVolumes.length}}
      <div data-test-client-host-volumes class="boxed-section">
        <div class="boxed-section-head">
          Host Volumes
        </div>
        <div class="boxed-section-body is-full-bleed">
          <ListTable
            @source={{@controller.sortedHostVolumes}}
            @class="is-striped"
            as |t|
          >
            <t.head>
              <th>
                Name
              </th>
              <th>
                Source
              </th>
              <th>
                Permissions
              </th>
            </t.head>
            <t.body as |row|>
              <tr data-test-client-host-volume>
                <td data-test-name>
                  {{row.model.name}}
                </td>
                <td data-test-path>
                  <code>
                    {{row.model.path}}
                  </code>
                </td>
                <td data-test-permissions>
                  {{if row.model.readOnly "Read" "Read/Write"}}
                </td>
              </tr>
            </t.body>
          </ListTable>
        </div>
      </div>
    {{/if}}
    <div data-test-driver-status class="boxed-section">
      <div class="boxed-section-head">
        Driver Status
      </div>
      <div class="boxed-section-body">
        <ListAccordion @source={{@controller.sortedDrivers}} @key="name" as |a|>
          <a.head
            @buttonLabel="details"
            @buttonType="client-detail"
            @isExpandable={{a.item.detected}}
          >
            <div
              class="columns inline-definitions
                {{unless a.item.detected 'is-faded'}}"
            >
              <div class="column is-1">
                <span data-test-name>
                  {{a.item.name}}
                </span>
              </div>
              <div class="column is-2">
                {{#if a.item.detected}}
                  <span data-test-health>
                    <span class="color-swatch {{a.item.healthClass}}"></span>
                    {{if a.item.healthy "Healthy" "Unhealthy"}}
                  </span>
                {{/if}}
              </div>
              <div class="column">
                <span class="pair">
                  <span class="term">
                    Detected
                  </span>
                  <span data-test-detected>
                    {{if a.item.detected "Yes" "No"}}
                  </span>
                </span>
                <span class="is-pulled-right">
                  <span class="pair">
                    <span class="term">
                      Last Updated
                    </span>
                    <span
                      data-test-last-updated
                      class="tooltip"
                      aria-label="{{formatTs a.item.updateTime}}"
                    >
                      {{momentFromNow a.item.updateTime interval=1000}}
                    </span>
                  </span>
                </span>
              </div>
            </div>
          </a.head>
          <a.body>
            <p data-test-health-description class="message">
              {{a.item.healthDescription}}
            </p>
            <div data-test-driver-attributes class="boxed-section">
              <div class="boxed-section-head">
                {{capitalize a.item.name}}
                Attributes
              </div>
              {{#if a.item.attributesShort}}
                <div class="boxed-section-body is-full-bleed">
                  <AttributesTable
                    @attributePairs={{a.item.attributesShort}}
                    @class="attributes-table"
                  />
                </div>
              {{else}}
                <div class="boxed-section-body">
                  <div class="empty-message">
                    <h3 class="empty-message-headline">
                      No Driver Attributes
                    </h3>
                  </div>
                </div>
              {{/if}}
            </div>
          </a.body>
        </ListAccordion>
      </div>
    </div>
    <div class="boxed-section">
      <div class="boxed-section-head">
        Attributes
      </div>
      <div class="boxed-section-body is-full-bleed">
        <AttributesTable
          data-test-attributes
          @attributePairs={{@model.attributes.structured.root}}
          @class="attributes-table"
          @copyable={{true}}
        />
      </div>
    </div>
    <div class="boxed-section">
      <div class="boxed-section-head">
        Meta
      </div>
      {{#if @controller.hasMeta}}
        <div class="boxed-section-body is-full-bleed">
          <AttributesTable
            data-test-meta
            @attributePairs={{@model.meta.structured.root}}
            @editable={{can "write client"}}
            @onKVSave={{@controller.addDynamicMetaData}}
            @onKVEdit={{@controller.validateMetadata}}
            @class="attributes-table"
          />
        </div>
      {{else}}
        <div class="boxed-section-body">
          <div data-test-empty-meta-message class="empty-message">
            <h3 class="empty-message-headline">
              No Meta Attributes
            </h3>
            <p class="empty-message-body">
              This client is configured with no meta attributes.
            </p>
          </div>
        </div>
      {{/if}}
      {{#if (can "write client")}}
        {{#if @controller.editingMetadata}}
          <div class="add-dynamic-metadata">
            <h3 class="title is-6">Add Dynamic Metadata</h3>
            <MetadataEditor
              @kv={{@controller.newMetaData}}
              @onEdit={{@controller.validateMetadata}}
            >
              <button
                data-test-new-metadata-button
                disabled={{or
                  (not @controller.newMetaData.key)
                  (not @controller.newMetaData.value)
                }}
                type="submit"
                class="button is-primary"
                {{on "click" @controller.saveEditingMetadata}}
              >
                Add
                {{@controller.newMetaData.key}}
                to node metadata
              </button>

              <button
                type="button"
                class="button is-secondary"
                {{on "click" @controller.cancelEditingMetadata}}
              >
                Cancel
              </button>

            </MetadataEditor>
          </div>
        {{else}}
          <div class="add-dynamic-metadata">
            <button
              type="button"
              class="button is-primary"
              {{on "click" @controller.beginEditingMetadata}}
              {{keyboardShortcut
                label="Add Dynamic Node Metadata"
                pattern=(array "m" "e" "t" "a")
                action=@controller.beginEditingMetadata
              }}
            >
              Add new Dynamic Metadata
            </button>
          </div>
        {{/if}}
      {{/if}}
    </div>
  </section>
</template>
