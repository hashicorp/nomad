/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { pageTitle } from 'ember-page-title';
import CopyButton from 'nomad-ui/components/copy-button';
import ListTable from 'nomad-ui/components/list-table';
import ServerSubnav from 'nomad-ui/components/server-subnav';

<template>
  {{pageTitle "Server " @model.name}}
  <ServerSubnav @server={{@model}} />
  <section class="section">
    <h1 data-test-title class="title">
      Server
      {{@model.name}}
      {{#if @model.isLeader}}
        <span
          data-test-leader-badge
          class="bumper-left tag is-primary"
        >Leader</span>
      {{/if}}
    </h1>
    <div class="boxed-section is-small">
      <div
        data-test-server-details
        class="boxed-section-body inline-definitions"
      >
        <span class="label">Server Details</span>
        <span data-test-status class="pair"><span class="term">Status</span>
          {{@model.status}}
        </span>
        <span data-test-address class="pair"><span class="term">Address</span>
          {{@model.rpcAddr}}
        </span>
        <span data-test-datacenter class="pair"><span
            class="term"
          >Datacenter</span>
          {{@model.datacenter}}
        </span>
      </div>
    </div>
    <div class="boxed-section">
      <div class="boxed-section-head">
        Server Tags
      </div>
      <div class="boxed-section-body is-full-bleed">
        <ListTable
          @source={{@controller.sortedTags}}
          @class="is-striped"
          as |t|
        >
          <t.head>
            <td class="is-one-quarter">Name</td>
            <td>Value</td>
          </t.head>
          <t.body as |row|>
            <tr data-test-server-tag>
              <td>{{row.model.name}}</td>
              <td>
                <CopyButton
                  @inset={{true}}
                  @compact={{true}}
                  @clipboardText={{row.model.value}}
                />
                {{row.model.value}}
              </td>
            </tr>
          </t.body>
        </ListTable>
      </div>
    </div>
  </section>
</template>
