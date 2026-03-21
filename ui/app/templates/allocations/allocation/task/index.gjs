/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { concat } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import {
  HdsAlert,
  HdsCodeBlock,
  HdsPageHeader,
  HdsSeparator,
} from '@hashicorp/design-system-components/components';
import { pageTitle } from 'ember-page-title';
import can from 'ember-can/helpers/can';
import perform from 'ember-concurrency/helpers/perform';
import eq from 'ember-truth-helpers/helpers/eq';
import ActionsDropdown from 'nomad-ui/components/actions-dropdown';
import ExecOpenButton from 'nomad-ui/components/exec/open-button';
import JobPagePartsMeta from 'nomad-ui/components/job-page/parts/meta';
import ListTable from 'nomad-ui/components/list-table';
import PrimaryMetricTask from 'nomad-ui/components/primary-metric/task';
import ProxyTag from 'nomad-ui/components/proxy-tag';
import TaskSubnav from 'nomad-ui/components/task-subnav';
import TwoStepButton from 'nomad-ui/components/two-step-button';
import formatTs from 'nomad-ui/helpers/format-ts';
import formatVolumeName from 'nomad-ui/helpers/format-volume-name';
import stringifyObject from 'nomad-ui/helpers/stringify-object';
import { and } from 'ember-truth-helpers';

<template>
  {{pageTitle "Task " @model.name}}
  <TaskSubnav @task={{@model}} />

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

    <HdsPageHeader class="job-page-header" as |PH|>
      <PH.Title data-test-title>
        {{@model.name}}
        {{#if @model.isConnectProxy}}
          <ProxyTag @class="bumper-left" />
        {{/if}}
        <span
          class="{{unless @model.isConnectProxy 'bumper-left'}}
            tag
            {{@model.stateClass}}"
          data-test-state
        >
          {{@model.state}}
        </span>
      </PH.Title>
      <PH.Actions>
        {{#if @model.isRunning}}

          {{#if @controller.shouldShowActions}}
            <ActionsDropdown
              @actions={{@model.task.actions}}
              @allocation={{@model.allocation}}
            />
          {{/if}}

          <div class="two-step-button">
            <ExecOpenButton
              @job={{@model.task.taskGroup.job}}
              @taskGroup={{@model.task.taskGroup}}
              @allocation={{@model.allocation}}
              @task={{@model.task}}
            />
          </div>

          <TwoStepButton
            data-test-restart
            @alignRight={{true}}
            @idleText="Restart Task"
            @cancelText="Cancel"
            @confirmText="Yes, Restart Task"
            @confirmationMessage="Are you sure? This will restart the task in-place."
            @awaitingConfirmation={{@controller.restartTask.isRunning}}
            @disabled={{@controller.restartTask.isRunning}}
            @onConfirm={{perform @controller.restartTask}}
          />
        {{/if}}
      </PH.Actions>
    </HdsPageHeader>

    {{#if @model.task.schedule}}
      <HdsAlert
        @type="inline"
        @icon="delay"
        @color="highlight"
        class="time-based-alert"
        as |A|
      >
        {{#if (eq @model.paused "")}}
          <A.Title>This task is currently running on schedule</A.Title>
          <A.Description>This task is running as per the defined schedule.</A.Description>
          <A.Button
            @text="Force Pause"
            @color="secondary"
            {{on "click" (perform @controller.forcePause)}}
          />
          <A.Button
            @text="Remove from Schedule"
            @color="secondary"
            {{on "click" (perform @controller.forceRun)}}
          />
        {{else if (eq @model.paused "scheduled_pause")}}
          <A.Title>This task is currently paused on schedule</A.Title>
          <A.Description>This task is paused and will resume on the next
            scheduled run.</A.Description>
          <A.Button
            @text="Force Run"
            @color="secondary"
            {{on "click" (perform @controller.forceRun)}}
          />
          <A.Button
            @text="Remove from Schedule"
            @color="secondary"
            {{on "click" (perform @controller.forcePause)}}
          />
        {{else if (eq @model.paused "force_pause")}}
          <A.Title>This task is manually paused</A.Title>
          <A.Description>This task has been paused manually and is not following
            the schedule.</A.Description>
          <A.Button
            @text="Force Run"
            @color="secondary"
            {{on "click" (perform @controller.forceRun)}}
          />
          <A.Button
            @text="Put Back on Schedule"
            @color="secondary"
            {{on "click" (perform @controller.reEnableSchedule)}}
          />
        {{else if (eq @model.paused "force_run")}}
          <A.Title>This task is manually running</A.Title>
          <A.Description>This task is running manually and is not following the
            schedule.</A.Description>
          <A.Button
            @text="Force Pause"
            @color="secondary"
            {{on "click" (perform @controller.forcePause)}}
          />
          <A.Button
            @text="Put Back on Schedule"
            @color="secondary"
            {{on "click" (perform @controller.reEnableSchedule)}}
          />
        {{/if}}
        <A.Generic>
          <HdsCodeBlock
            @value={{stringifyObject @model.task.schedule.cron}}
            @hasLineNumbers={{false}}
            @language="hcl"
          />
        </A.Generic>
      </HdsAlert>
      <HdsSeparator />
    {{/if}}

    <div class="boxed-section is-small">
      <div class="boxed-section-body inline-definitions">
        <span class="label">
          Task Details
        </span>
        <span class="pair" data-test-started-at>
          <span class="term">
            Started At
          </span>
          {{formatTs @model.startedAt}}
        </span>
        {{#if @model.finishedAt}}
          <span class="pair">
            <span class="term">
              Finished At
            </span>
            {{formatTs @model.finishedAt}}
          </span>
        {{/if}}
        <span class="pair">
          <span class="term">
            Driver
          </span>
          {{@model.task.driver}}
        </span>
        <span class="pair">
          <span class="term">
            Lifecycle
          </span>
          <span data-test-lifecycle>
            {{@model.task.lifecycleName}}
          </span>
        </span>
        <span class="pair">
          <span class="term">
            Namespace
          </span>
          <span>
            {{@model.allocation.job.namespace.name}}
          </span>
        </span>

        {{#if (and (can "list variables") @model.task.pathLinkedVariable)}}
          <span class="pair" data-test-task-stat="variables">
            <LinkTo
              @route="variables.variable"
              @model={{@model.task.pathLinkedVariable.id}}
            >Variables</LinkTo>
          </span>
        {{/if}}

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
              <PrimaryMetricTask @taskState={{@model}} @metric="cpu" />
            </div>
            <div class="column">
              <PrimaryMetricTask @taskState={{@model}} @metric="memory" />
            </div>
          </div>
        {{else}}
          <div data-test-resource-error class="empty-message">
            <h3
              data-test-resource-error-headline
              class="empty-message-headline"
            >
              Task isn't running
            </h3>
            <p class="empty-message-body">
              Only running tasks utilize resources.
            </p>
          </div>
        {{/if}}
      </div>
    </div>

    {{#if @model.task.volumeMounts.length}}
      <div data-test-volumes class="boxed-section">
        <div class="boxed-section-head">
          Volumes
        </div>
        <div class="boxed-section-body is-full-bleed">
          <ListTable @source={{@model.task.volumeMounts}} as |t|>
            <t.head>
              <th>
                Name
              </th>
              <th>
                Destination
              </th>
              <th>
                Permissions
              </th>
              <th>
                Client Source
              </th>
            </t.head>
            <t.body as |row|>
              <tr data-test-volume>
                <td data-test-volume-name>
                  {{row.model.volume}}
                </td>
                <td data-test-volume-destination>
                  <code>
                    {{row.model.destination}}
                  </code>
                </td>
                <td data-test-volume-permissions>
                  {{if row.model.readOnly "Read" "Read/Write"}}
                </td>
                <td data-test-volume-client-source>
                  {{#if row.model.isCSI}}
                    <LinkTo
                      @route="storage.volumes.volume"
                      @model={{concat
                        (formatVolumeName
                          source=row.model.source
                          isPerAlloc=row.model.volumeDeclaration.perAlloc
                          volumeExtension=@model.allocation.volumeExtension
                        )
                        "@"
                        row.model.namespace.id
                      }}
                    >
                      {{formatVolumeName
                        source=row.model.source
                        isPerAlloc=row.model.volumeDeclaration.perAlloc
                        volumeExtension=@model.allocation.volumeExtension
                      }}
                    </LinkTo>
                  {{else}}
                    {{row.model.source}}
                  {{/if}}
                </td>
              </tr>
            </t.body>
          </ListTable>
        </div>
      </div>
    {{/if}}

    <div class="boxed-section">
      <div class="boxed-section-head">
        Recent Events
      </div>
      <div class="boxed-section-body is-full-bleed">
        <ListTable
          @source={{@controller.recentEvents}}
          @class="is-striped recent-events-table"
          as |t|
        >
          <t.head>
            <th class="is-3">
              Time
            </th>
            <th class="is-1">
              Type
            </th>
            <th>
              Description
            </th>
          </t.head>
          <t.body as |row|>
            <tr data-test-task-event>
              <td data-test-task-event-time>
                {{formatTs row.model.time}}
              </td>
              <td data-test-task-event-type>
                {{row.model.type}}
              </td>
              <td data-test-task-event-message>
                {{#if row.model.message}}
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

    {{#if @model.task.meta}}
      <JobPagePartsMeta @meta={{@model.task.meta}} />
    {{/if}}
  </section>
</template>
