/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { pageTitle } from 'ember-page-title';
import eq from 'ember-truth-helpers/helpers/eq';
import JobSubnav from 'nomad-ui/components/job-subnav';
import ListTable from 'nomad-ui/components/list-table';
import formatMonthTs from 'nomad-ui/helpers/format-month-ts';

<template>
  {{pageTitle "Job " @model.name " evaluations"}}
  <JobSubnav @job={{@model}} />

  <section class="section">
    {{#if @controller.sortedEvaluations.length}}
      <ListTable
        @source={{@controller.sortedEvaluations}}
        @sortProperty={{@controller.sortProperty}}
        @sortDescending={{@controller.sortDescending}}
        as |t|
      >
        <t.head>
          <th>ID</th>
          <t.sortBy @prop="priority">Priority</t.sortBy>
          <t.sortBy @prop="createTime">Created</t.sortBy>
          <t.sortBy @prop="triggeredBy">Triggered By</t.sortBy>
          <t.sortBy @prop="status">Status</t.sortBy>
          <t.sortBy @prop="hasPlacementFailures">Placement Failures</t.sortBy>
        </t.head>
        <t.body as |row|>
          <tr data-test-evaluation="{{row.model.shortId}}">
            <td data-test-id>{{row.model.shortId}}</td>
            <td data-test-priority>{{row.model.priority}}</td>
            <td data-test-create-time>{{formatMonthTs
                row.model.createTime
              }}</td>
            <td data-test-triggered-by>{{row.model.triggeredBy}}</td>
            <td data-test-status>{{row.model.status}}</td>
            <td data-test-blocked>
              {{#if (eq row.model.status "blocked")}}
                N/A - In Progress
              {{else if row.model.hasPlacementFailures}}
                True
              {{else}}
                False
              {{/if}}
            </td>
          </tr>
        </t.body>
      </ListTable>
    {{else}}
      <div data-test-empty-evaluations-list class="empty-message">
        <h3
          data-test-empty-evaluations-list-headline
          class="empty-message-headline"
        >No Evaluations</h3>
        <p class="empty-message-body">This is most likely due to garbage
          collection.</p>
      </div>
    {{/if}}
  </section>
</template>
