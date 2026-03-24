/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { fn, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import { pageTitle } from 'ember-page-title';
import perform from 'ember-concurrency/helpers/perform';
import and from 'ember-truth-helpers/helpers/and';
import eq from 'ember-truth-helpers/helpers/eq';
import or from 'ember-truth-helpers/helpers/or';
import AllocationRow from 'nomad-ui/components/allocation-row';
import AllocationServiceSidebar from 'nomad-ui/components/allocation-service-sidebar';
import AllocationSubnav from 'nomad-ui/components/allocation-subnav';
import CopyButton from 'nomad-ui/components/copy-button';
import ExecOpenButton from 'nomad-ui/components/exec/open-button';
import LifecycleChart from 'nomad-ui/components/lifecycle-chart';
import ListTable from 'nomad-ui/components/list-table';
import PrimaryMetricAllocation from 'nomad-ui/components/primary-metric/allocation';
import RescheduleEventTimeline from 'nomad-ui/components/reschedule-event-timeline';
import ServiceStatusBar from 'nomad-ui/components/service-status-bar';
import TaskRow from 'nomad-ui/components/task-row';
import Tooltip from 'nomad-ui/components/tooltip';
import TwoStepButton from 'nomad-ui/components/two-step-button';
import formatJobId from 'nomad-ui/helpers/format-job-id';
import formatScheduledBytes from 'nomad-ui/helpers/format-scheduled-bytes';
import formatScheduledHertz from 'nomad-ui/helpers/format-scheduled-hertz';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';

<template>
  {{pageTitle "Allocation " @model.name}}
  <AllocationSubnav @allocation={{@model}} />

  <section class="section">
    {{#if @controller.error}}
      <div data-test-inline-error class="notification is-danger">
        <div class="columns">
          <div class="column">
            <h3 data-test-inline-error-title class="title is-4">
              {{@controller.error.title}}
            </h3>
            <p data-test-inline-error-body>
              {{@controller.error.description}}
            </p>
          </div>
          <div class="column is-centered is-minimum">
            <button
              data-test-inline-error-close
              class="button is-danger"
              {{on "click" @controller.onDismiss}}
              type="button"
            >
              Okay
            </button>
          </div>
        </div>
      </div>
    {{/if}}

    <h1 data-test-title class="title with-headroom with-flex">
      <div>
        Allocation
        {{@model.name}}
        <span class="bumper-left tag {{@model.statusClass}}">
          {{@model.clientStatus}}
        </span>
      </div>
      <div>
        {{#if @model.isRunning}}
          <div class="two-step-button">
            <ExecOpenButton @job={{@model.job}} @allocation={{@model}} />
          </div>
          <TwoStepButton
            data-test-stop
            @alignRight={{true}}
            @idleText="Stop Alloc"
            @cancelText="Cancel Stop"
            @confirmText="Yes, Stop Alloc"
            @confirmationMessage="Are you sure? This will reschedule the allocation on a different client."
            @awaitingConfirmation={{@controller.stopAllocation.isRunning}}
            @disabled={{or
              @controller.stopAllocation.isRunning
              @controller.restartAllocation.isRunning
            }}
            @onConfirm={{perform @controller.stopAllocation}}
          />
          <TwoStepButton
            data-test-restart
            @alignRight={{true}}
            @idleText="Restart Alloc"
            @cancelText="Cancel Restart"
            @confirmText="Yes, Restart Alloc"
            @confirmationMessage="Are you sure? This will restart the tasks that are currently running in-place."
            @awaitingConfirmation={{@controller.restartAllocation.isRunning}}
            @disabled={{or
              @controller.stopAllocation.isRunning
              @controller.restartAllocation.isRunning
            }}
            @onConfirm={{perform @controller.restartAllocation}}
          />
          <TwoStepButton
            data-test-restart-all
            @alignRight={{true}}
            @idleText="Restart All Tasks"
            @cancelText="Cancel Restart"
            @confirmText="Yes, Restart All Tasks"
            @confirmationMessage="Are you sure? This will restart all tasks in-place."
            @awaitingConfirmation={{@controller.restartAllocation.isRunning}}
            @disabled={{or
              @controller.stopAllocation.isRunning
              @controller.restartAllocation.isRunning
            }}
            @onConfirm={{perform @controller.restartAll}}
          />
        {{/if}}
      </div>
    </h1>

    <span class="tag is-hollow is-small is-alone no-text-transform">
      {{@model.id}}
      <CopyButton @clipboardText={{@model.id}} />
    </span>

    <div class="boxed-section is-small">
      <div
        data-test-allocation-details
        class="boxed-section-body inline-definitions"
      >
        <span class="label">
          Allocation Details
        </span>
        <span class="pair job-link">
          <span class="term">
            Job
          </span>
          <LinkTo
            @route="jobs.job"
            @model={{formatJobId @model.job.id}}
            data-test-job-link
          >
            {{@model.job.name}}
          </LinkTo>
        </span>
        <span class="pair node-link">
          <span class="term">
            Client
          </span>
          <Tooltip @text={{@model.node.name}}>
            <LinkTo
              @route="clients.client"
              @model={{@model.node.id}}
              data-test-client-link
            >
              {{@model.node.shortId}}
            </LinkTo>
          </Tooltip>
        </span>
        <span class="pair">
          <span class="term">
            Namespace
          </span>
          <span>
            {{@model.job.namespace.name}}
          </span>
        </span>
      </div>
    </div>

    <div class="boxed-section">
      <div class="boxed-section-head is-hollow">
        Resource Utilization
      </div>
      <div class="boxed-section-body">
        {{#if @model.isRunning}}
          <div class="columns">
            <div class="column">
              <PrimaryMetricAllocation @allocation={{@model}} @metric="cpu" />
            </div>
            <div class="column">
              <PrimaryMetricAllocation
                @allocation={{@model}}
                @metric="memory"
              />
            </div>
          </div>
        {{else}}
          <div data-test-resource-error class="empty-message">
            <h3
              data-test-resource-error-headline
              class="empty-message-headline"
            >
              Allocation isn't running
            </h3>
            <p class="empty-message-body">
              Only running allocations utilize resources.
            </p>
          </div>
        {{/if}}
      </div>
    </div>

    <LifecycleChart @taskStates={{@model.states}} />

    <div class="boxed-section">
      <div class="boxed-section-head">
        Tasks
      </div>
      <div
        class="boxed-section-body
          {{if @controller.sortedStates.length 'is-full-bleed'}}"
      >
        {{#if @controller.sortedStates.length}}
          <ListTable
            @source={{@controller.sortedStates}}
            @sortProperty={{@controller.sortProperty}}
            @sortDescending={{@controller.sortDescending}}
            @class="is-striped"
            as |t|
          >
            <t.head>
              <th class="is-narrow"><span class="visually-hidden">Driver Health</span></th>
              <t.sortBy @prop="name">
                Name
              </t.sortBy>
              <t.sortBy @prop="state">
                State
              </t.sortBy>
              <th>
                Last Event
              </th>
              <t.sortBy @prop="events.lastObject.time">
                Time
              </t.sortBy>
              <th>
                Volumes
              </th>
              <th>
                CPU
              </th>
              <th>
                Memory
              </th>
            </t.head>
            <t.body as |row|>
              <TaskRow
                {{keyboardShortcut
                  enumerated=true
                  action=(fn
                    @controller.taskClick row.model.allocation row.model
                  )
                }}
                data-test-task-row={{row.model.name}}
                @task={{row.model}}
                @onClick={{fn
                  @controller.taskClick
                  row.model.allocation
                  row.model
                }}
              />
            </t.body>
          </ListTable>
        {{else}}
          <div data-test-empty-tasks-list class="empty-message">
            <h3
              data-test-empty-tasks-list-headline
              class="empty-message-headline"
            >
              No Tasks
            </h3>
            <p data-test-empty-tasks-list-body class="empty-message-body">
              Allocations will not have tasks until they are in a running state.
            </p>
          </div>
        {{/if}}
      </div>
    </div>

    {{#if @controller.ports.length}}
      <div class="boxed-section" data-test-allocation-ports>
        <div class="boxed-section-head">
          Ports
        </div>
        <div class="boxed-section-body is-full-bleed">
          <ListTable @source={{@controller.ports}} as |t|>
            <t.head>
              <th>
                Name
              </th>
              <th>
                Host Address
              </th>
              <th>
                Mapped Port
              </th>
            </t.head>
            <t.body as |row|>
              <tr data-test-allocation-port>
                <td data-test-allocation-port-name>
                  {{row.model.label}}
                </td>
                <td data-test-allocation-port-address>
                  <a
                    href="http://{{row.model.hostIp}}:{{row.model.value}}"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    {{row.model.hostIp}}:{{row.model.value}}
                  </a>
                </td>
                <td data-test-allocation-port-to>
                  {{row.model.to}}
                </td>
              </tr>
            </t.body>
          </ListTable>
        </div>
      </div>
    {{/if}}

    {{#if @controller.servicesWithHealthChecks.length}}
      <div class="boxed-section">
        <div class="boxed-section-head">
          Services
        </div>
        <div class="boxed-section-body is-full-bleed">
          <ListTable
            class="allocation-services-table"
            @source={{@controller.servicesWithHealthChecks}}
            as |t|
          >
            <t.head>
              <th class="is-narrow"><span class="visually-hidden">Service Type</span></th>
              <th>
                Name
              </th>
              <th>
                Port
              </th>
              <td>
                Tags
              </td>
              <td>
                Health Check Status
              </td>
            </t.head>
            <t.body as |row|>
              <tr
                data-test-service
                class="is-interactive
                  {{if (eq @controller.activeService row.model) 'is-active'}}"
                {{on "click" (fn @controller.handleServiceClick row.model)}}
                {{keyboardShortcut
                  enumerated=true
                  action=(fn @controller.handleServiceClick row.model)
                }}
              >
                <td class="is-narrow">
                  {{#if (eq row.model.provider "nomad")}}
                    <HdsIcon @name="nomad-color" @isInline={{true}} />
                  {{else}}
                    <HdsIcon @name="consul-color" @isInline={{true}} />
                    {{#if row.model.connect}}
                      <HdsIcon
                        @name="mesh"
                        @color="#444444"
                        @isInline={{true}}
                      />
                    {{/if}}
                  {{/if}}
                </td>
                <td data-test-service-name class="is-long-text">
                  {{row.model.name}}
                </td>
                <td data-test-service-port>
                  {{row.model.portLabel}}
                </td>
                <td data-test-service-tags class="is-long-text">
                  {{#each row.model.tags as |tag|}}
                    <span class="tag is-service">{{tag}}</span>
                  {{/each}}
                  {{#each row.model.canary_tags as |tag|}}
                    <span class="tag canary is-service">{{tag}}</span>
                  {{/each}}
                </td>
                <td data-test-service-health class="is-2">
                  {{#if (eq row.model.provider "nomad")}}
                    <div class="inline-chart">
                      <ServiceStatusBar
                        @isNarrow={{true}}
                        @status={{row.model.mostRecentCheckStatus}}
                      />
                    </div>
                  {{/if}}
                </td>
              </tr>
            </t.body>
          </ListTable>
        </div>
      </div>
    {{/if}}

    {{#if @model.hasRescheduleEvents}}
      <div class="boxed-section" data-test-reschedule-events>
        <div class="boxed-section-head is-hollow">
          Reschedule Events
        </div>
        <div class="boxed-section-body">
          <RescheduleEventTimeline @allocation={{@model}} />
        </div>
      </div>
    {{/if}}

    {{#if @model.wasPreempted}}
      <div class="boxed-section is-warning" data-test-was-preempted>
        <div class="boxed-section-head">
          Preempted By
        </div>
        <div class="boxed-section-body">
          {{#if @controller.preempter}}
            <div class="boxed-section is-small">
              <div class="boxed-section-body inline-definitions">
                <span class="pair">
                  <span
                    data-test-allocation-status
                    class="tag {{@controller.preempter.statusClass}}"
                  >
                    {{@controller.preempter.clientStatus}}
                  </span>
                </span>
                <span class="pair">
                  <span class="term" data-test-allocation-name>
                    {{@controller.preempter.name}}
                  </span>
                  <LinkTo
                    @route="allocations.allocation"
                    @model={{@controller.preempter}}
                    data-test-allocation-id
                  >
                    {{@controller.preempter.shortId}}
                  </LinkTo>
                </span>
                <span class="pair job-link">
                  <span class="term">
                    Job
                  </span>
                  <LinkTo
                    @route="jobs.job"
                    @model={{@controller.preempter.job}}
                    data-test-job-link
                  >
                    {{@controller.preempter.job.name}}
                  </LinkTo>
                </span>
                <span class="pair job-priority">
                  <span class="term">
                    Priority
                  </span>
                  <span data-test-job-priority>
                    {{@controller.preempter.job.priority}}
                  </span>
                </span>
                <span class="pair node-link">
                  <span class="term">
                    Client
                  </span>
                  <LinkTo
                    @route="clients.client"
                    @model={{@controller.preempter.node}}
                    data-test-client-link
                  >
                    {{@controller.preempter.node.shortId}}
                  </LinkTo>
                </span>
                <span class="pair">
                  <span class="term">
                    Reserved CPU
                  </span>
                  <span data-test-allocation-cpu>
                    {{formatScheduledHertz @controller.preempter.resources.cpu}}
                  </span>
                </span>
                <span class="pair">
                  <span class="term">
                    Reserved Memory
                  </span>
                  <span data-test-allocation-memory>
                    {{formatScheduledBytes
                      @controller.preempter.resources.memory
                      start="MiB"
                    }}
                  </span>
                </span>
              </div>
            </div>
          {{else}}
            <div class="empty-message">
              <h3 class="empty-message-headline">
                Allocation is gone
              </h3>
              <p class="empty-message-body">
                This allocation has been stopped and garbage collected.
              </p>
            </div>
          {{/if}}
        </div>
      </div>
    {{/if}}

    {{#if
      (and
        @model.preemptedAllocations.isFulfilled
        @model.preemptedAllocations.length
      )
    }}
      <div class="boxed-section" data-test-preemptions>
        <div class="boxed-section-head">
          Preempted Allocations
        </div>
        <div class="boxed-section-body">
          <ListTable
            @source={{@model.preemptedAllocations}}
            @class="allocations is-isolated"
            as |t|
          >
            <t.head>
              <th class="is-narrow"><span class="visually-hidden">Driver Health,
                  Scheduling, and Preemption</span></th>
              <th>
                ID
              </th>
              <th>
                Task Group
              </th>
              <th>
                Created
              </th>
              <th>
                Modified
              </th>
              <th>
                Status
              </th>
              <th>
                Version
              </th>
              <th>
                Node
              </th>
              <th>
                CPU
              </th>
              <th>
                Memory
              </th>
            </t.head>
            <t.body as |row|>
              <AllocationRow
                @allocation={{row.model}}
                @context="job"
                data-test-allocation={{row.model.id}}
              />
            </t.body>
          </ListTable>
        </div>
      </div>
    {{/if}}

    <AllocationServiceSidebar
      @service={{@controller.activeService}}
      @allocation={{@model}}
      @fns={{hash closeSidebar=@controller.closeSidebar}}
    />
  </section>
</template>
