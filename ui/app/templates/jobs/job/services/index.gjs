/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import ListTable from 'nomad-ui/components/list-table';
import JobServiceRow from 'nomad-ui/components/job-service-row';

<template>
  <section class="section service-list">
    {{#if @controller.sortedServices.length}}
      <ListTable
        @source={{@controller.sortedServices}}
        @sortProperty={{@controller.sortProperty}}
        @sortDescending={{@controller.sortDescending}}
        as |t|
      >
        <t.head>
          <t.sortBy @prop="name">Name</t.sortBy>
          <t.sortBy @prop="level">Level</t.sortBy>
          <th>Tags</th>
          <t.sortBy @prop="numAllocs">Number of Allocations</t.sortBy>
        </t.head>
        <t.body as |row|>
          <JobServiceRow @service={{row.model}} />
        </t.body>
      </ListTable>
    {{else}}
      <div class="boxed-section-body">
        <div class="empty-message" data-test-empty-services-list>
          <h3
            class="empty-message-headline"
            data-test-empty-services-list-headline
          >No Services</h3>
          <p class="empty-message-body">
            No services running on
            {{@controller.job.name}}.
          </p>
        </div>
      </div>
    {{/if}}
  </section>
</template>
