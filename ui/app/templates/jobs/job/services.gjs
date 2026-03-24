/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import JobSubnav from 'nomad-ui/components/job-subnav';
import { pageTitle } from 'ember-page-title';

<template>
  {{pageTitle "Job " @model.name " services"}}
  <JobSubnav @job={{@model}} />
  {{outlet}}
</template>
