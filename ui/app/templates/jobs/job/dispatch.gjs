/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import { pageTitle } from 'ember-page-title';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import JobDispatch from 'nomad-ui/components/job-dispatch';
import JobSubnav from 'nomad-ui/components/job-subnav';

<template>
  <Breadcrumb
    @crumb={{hash label="Dispatch" args=(array "jobs.job.dispatch")}}
  />
  {{pageTitle "Dispatch new " @model.name}}
  <JobSubnav @job={{@model}} />
  <section class="section">
    <JobDispatch @job={{@model}} />
  </section>
</template>
