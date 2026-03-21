/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import can from 'ember-can/helpers/can';
import { pageTitle } from 'ember-page-title';
import AgentMonitor from 'nomad-ui/components/agent-monitor';
import ForbiddenMessage from 'nomad-ui/components/forbidden-message';
import ServerSubnav from 'nomad-ui/components/server-subnav';

<template>
  {{pageTitle "Server " @model.name}}
  <ServerSubnav @server={{@model}} />
  <section class="section is-full-width">
    {{#if (can "read agent")}}
      <AgentMonitor
        @level={{@controller.level}}
        @server={{@model}}
        @onLevelChange={{@controller.setLevel}}
      />
    {{else}}
      <ForbiddenMessage @permission="agent:read" />
    {{/if}}
  </section>
</template>
