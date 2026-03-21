/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, fn } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import can from 'ember-can/helpers/can';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import {
  HdsButton,
  HdsTable,
} from '@hashicorp/design-system-components/components';

<template>
  <section class="section">
    <header class="acl-explainer">
      <p>
        Namespaces allow jobs and associated objects to be segmented from each
        other and other users of the cluster.
      </p>
      <div>
        {{#if (can "write namespace")}}
          <HdsButton
            @text="Create Namespace"
            @icon="plus"
            @route="administration.namespaces.new"
            {{keyboardShortcut
              pattern=(array "n" "n")
              action=@controller.goToNewNamespace
              label="Create Namespace"
            }}
            data-test-create-namespace
          />
        {{else}}
          <HdsButton
            @text="Create Namespace"
            @icon="plus"
            disabled
            data-test-disabled-create-namespace
          />
        {{/if}}
      </div>
    </header>

    <HdsTable
      @caption="A list of namespaces for this cluster"
      class="acl-table"
      @model={{@model.namespaces}}
      @columns={{@controller.columns}}
      @sortBy="name"
    >
      <:body as |B|>
        <B.Tr
          {{keyboardShortcut
            enumerated=true
            action=(fn @controller.openNamespace B.data)
          }}
          data-test-namespace-row
        >
          <B.Td>
            <LinkTo
              data-test-namespace-name={{B.data.name}}
              @route="administration.namespaces.acl-namespace"
              @model={{B.data.name}}
            >{{B.data.name}}</LinkTo>
          </B.Td>
          <B.Td>{{B.data.description}}</B.Td>
        </B.Tr>
      </:body>
    </HdsTable>
  </section>
</template>
