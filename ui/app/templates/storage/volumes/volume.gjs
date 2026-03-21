/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { fn } from '@ember/helper';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import ListTable from 'nomad-ui/components/list-table';
import AllocationRow from 'nomad-ui/components/allocation-row';
import { pageTitle } from 'ember-page-title';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';

<template>
  {{#each @controller.breadcrumbs as |crumb|}}
    <Breadcrumb @crumb={{crumb}} />
  {{/each}}
  {{pageTitle "CSI Volume " @model.name}}
  {{! TODO: determine if /volumes/volume will just be CSI or if we ought to generalize it }}
  <section class="section with-headspace">
    <h1 class="title" data-test-title>{{@model.name}}</h1>

    <div class="boxed-section is-small">
      <div class="boxed-section-body inline-definitions">
        <span class="label">Volume Details</span>
        <span class="pair" data-test-volume-health>
          <span class="term">Health</span>
          {{if @model.schedulable "Schedulable" "Unschedulable"}}
        </span>
        <span class="pair" data-test-volume-provider>
          <span class="term">Provider</span>
          {{@model.provider}}
        </span>
        <span class="pair" data-test-volume-external-id>
          <span class="term">External ID</span>
          {{@model.externalId}}
        </span>
        {{#if @controller.system.shouldShowNamespaces}}
          <span class="pair" data-test-volume-namespace>
            <span class="term">Namespace</span>
            {{@model.namespace.name}}
          </span>
        {{/if}}
      </div>
    </div>

    <div class="boxed-section">
      <div class="boxed-section-head">
        Write Allocations
      </div>
      <div
        class="boxed-section-body
          {{if @model.writeAllocations.length 'is-full-bleed'}}"
      >
        {{#if @model.writeAllocations.length}}
          <ListTable
            @source={{@controller.sortedWriteAllocations}}
            @class="with-foot"
            as |t|
          >
            <t.head>
              <th class="is-narrow"><span class="visually-hidden">Driver Health,
                  Scheduling, and Preemption</span></th>
              <th>ID</th>
              <th>Created</th>
              <th>Modified</th>
              <th>Status</th>
              <th>Client</th>
              <th>Job</th>
              <th>Version</th>
              <th>CPU</th>
              <th>Memory</th>
            </t.head>
            <t.body as |row|>
              <AllocationRow
                {{keyboardShortcut
                  enumerated=true
                  action=(fn @controller.gotoAllocation row.model)
                }}
                data-test-write-allocation={{row.model.id}}
                @allocation={{row.model}}
                @context="volume"
                @onClick={{fn @controller.gotoAllocation row.model}}
              />
            </t.body>
          </ListTable>
        {{else}}
          <div class="empty-message" data-test-empty-write-allocations>
            <h3
              class="empty-message-headline"
              data-test-empty-write-allocations-headline
            >No Write Allocations</h3>
            <p
              class="empty-message-body"
              data-test-empty-write-allocations-message
            >No allocations are depending on this volume for read/write access.</p>
          </div>
        {{/if}}
      </div>
    </div>

    <div class="boxed-section">
      <div class="boxed-section-head">
        Read Allocations
      </div>
      <div
        class="boxed-section-body
          {{if @model.readAllocations.length 'is-full-bleed'}}"
      >
        {{#if @model.readAllocations.length}}
          <ListTable
            @source={{@controller.sortedReadAllocations}}
            @class="with-foot"
            as |t|
          >
            <t.head>
              <th class="is-narrow"><span class="visually-hidden">Driver Health,
                  Scheduling, and Preemption</span></th>
              <th>ID</th>
              <th>Modified</th>
              <th>Created</th>
              <th>Status</th>
              <th>Client</th>
              <th>Job</th>
              <th>Version</th>
              <th>CPU</th>
              <th>Memory</th>
            </t.head>
            <t.body as |row|>
              <AllocationRow
                {{keyboardShortcut
                  enumerated=true
                  action=(fn @controller.gotoAllocation row.model)
                }}
                data-test-read-allocation={{row.model.id}}
                @allocation={{row.model}}
                @context="volume"
                @onClick={{fn @controller.gotoAllocation row.model}}
              />
            </t.body>
          </ListTable>
        {{else}}
          <div class="empty-message" data-test-empty-read-allocations>
            <h3
              class="empty-message-headline"
              data-test-empty-read-allocations-headline
            >No Read Allocations</h3>
            <p
              class="empty-message-body"
              data-test-empty-read-allocations-message
            >No allocations are depending on this volume for read-only access.</p>
          </div>
        {{/if}}
      </div>
    </div>

    <div class="boxed-section">
      <div class="boxed-section-head">
        Capabilities
      </div>
      <div class="boxed-section-body is-full-bleed">
        <table class="table">
          <thead>
            <th>Setting</th>
            <th>Value</th>
          </thead>
          <tbody>
            <tr>
              <td>Access Mode</td>
              <td data-test-access-mode>{{@model.accessMode}}</td>
            </tr>
            <tr>
              <td>Attachment Mode</td>
              <td data-test-attachment-mode>{{@model.attachmentMode}}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </section>
</template>
