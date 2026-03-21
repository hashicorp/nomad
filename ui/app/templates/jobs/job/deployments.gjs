/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { pageTitle } from 'ember-page-title';
import JobDeploymentsStream from 'nomad-ui/components/job-deployments-stream';
import JobSubnav from 'nomad-ui/components/job-subnav';

<template>
  {{pageTitle "Job " @model.name " deployments"}}
  <JobSubnav @job={{@model}} />
  <section class="section">
    <JobDeploymentsStream @deployments={{@model.deployments}} />
  </section>
</template>
