/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { pageTitle } from 'ember-page-title';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import ForbiddenMessage from 'nomad-ui/components/forbidden-message';

<template>
  {{pageTitle "Variables: " @model.path}}
  {{#each @controller.breadcrumbs as |crumb|}}
    <Breadcrumb @crumb={{crumb}} />
  {{/each}}
  <section class="section single-variable">
    {{#if @controller.isForbidden}}
      <ForbiddenMessage />
    {{else}}
      {{outlet}}
    {{/if}}
  </section>
</template>
