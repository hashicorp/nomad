/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { fn } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import ListTable from 'nomad-ui/components/list-table';
import Tooltip from 'nomad-ui/components/tooltip';
import asyncEscapeHatch from 'nomad-ui/helpers/async-escape-hatch';
import formatId from 'nomad-ui/helpers/format-id';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';

<template>
  <section class="section service-list">
    <h1 class="title">
      <LinkTo class="back-link" @route="jobs.job.services">
        <HdsIcon
          @name="chevron-left"
          @title="Back to services"
          @size="24"
          @isInline={{true}}
        />
      </LinkTo>
      {{@model.name}}
    </h1>

    <ListTable @source={{@model.instances}} as |t|>
      <t.head>
        <th>Allocation</th>
        <th>Client</th>
        <th>IP Address &amp; Port</th>
      </t.head>
      <t.body as |row|>
        <tr data-test-service-row>
          {{#let (formatId row.model "allocation") as |allocation|}}
            <td
              {{keyboardShortcut
                enumerated=true
                action=(fn @controller.gotoAllocation row.model.allocation)
              }}
            >
              <LinkTo
                @route="allocations.allocation"
                @model={{allocation.id}}
              >{{allocation.shortId}}</LinkTo>
            </td>
          {{/let}}
          {{#let (asyncEscapeHatch row.model "node") as |node|}}
            <td>
              <Tooltip @text={{node.name}}>
                <LinkTo
                  @route="clients.client"
                  @model={{node.id}}
                >{{node.shortId}}</LinkTo>
              </Tooltip>
            </td>
          {{/let}}
          <td>
            {{row.model.address}}:{{row.model.port}}
          </td>
        </tr>
      </t.body>
    </ListTable>
  </section>
</template>
