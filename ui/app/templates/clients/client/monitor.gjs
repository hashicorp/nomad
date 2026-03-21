/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import can from 'ember-can/helpers/can';
import { pageTitle } from 'ember-page-title';
import { or } from 'ember-truth-helpers';
import AgentMonitor from 'nomad-ui/components/agent-monitor';
import ClientSubnav from 'nomad-ui/components/client-subnav';
import ForbiddenMessage from 'nomad-ui/components/forbidden-message';

<template>
  {{pageTitle "Client " (or @model.name @model.shortId)}}
  <ClientSubnav @client={{@model}} />
  <section class="section is-full-width">
    {{#if (can "read agent")}}
      <AgentMonitor
        @level={{@controller.level}}
        @client={{@model}}
        @onLevelChange={{@controller.setLevel}}
      />
    {{else}}
      <ForbiddenMessage @permission="agent:read" />
    {{/if}}
  </section>
</template>
