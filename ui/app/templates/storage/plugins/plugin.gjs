/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Breadcrumb from 'nomad-ui/components/breadcrumb';

<template>
  {{#each @controller.breadcrumbs as |crumb|}}
    <Breadcrumb @crumb={{crumb}} />
  {{/each}}
  {{outlet}}
</template>
