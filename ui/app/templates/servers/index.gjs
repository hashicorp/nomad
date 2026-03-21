/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { pageTitle } from 'ember-page-title';
import ForbiddenMessage from 'nomad-ui/components/forbidden-message';
import ListPagination from 'nomad-ui/components/list-pagination';
import ListTable from 'nomad-ui/components/list-table';
import ServerAgentRow from 'nomad-ui/components/server-agent-row';

<template>
  {{pageTitle "Servers"}}
  <section class="section">
    {{#if @controller.isForbidden}}
      <ForbiddenMessage />
    {{else}}
      <ListPagination
        @source={{@controller.sortedAgents}}
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
            <t.sortBy @prop="name">Name</t.sortBy>
            <t.sortBy @prop="status">Status</t.sortBy>
            <t.sortBy @prop="isLeader">Leader</t.sortBy>
            <t.sortBy
              @class="is-200px is-truncatable"
              @prop="address"
            >Address</t.sortBy>
            <t.sortBy @prop="serfPort">port</t.sortBy>
            <t.sortBy @prop="datacenter">Datacenter</t.sortBy>
            <t.sortBy @prop="version">Version</t.sortBy>
          </t.head>
          <t.body as |row|>
            <ServerAgentRow data-test-server-agent-row @agent={{row.model}} />
          </t.body>
        </ListTable>
        <div class="table-foot">
          <nav class="pagination">
            <div class="pagination-numbers">
              {{p.startsAt}}&ndash;{{p.endsAt}}
              of
              {{@controller.sortedAgents.length}}
            </div>
            <p.prev @class="pagination-previous"> &lt; </p.prev>
            <p.next @class="pagination-next"> &gt; </p.next>
            <ul class="pagination-list"></ul>
          </nav>
        </div>
      </ListPagination>
    {{/if}}
  </section>
</template>
