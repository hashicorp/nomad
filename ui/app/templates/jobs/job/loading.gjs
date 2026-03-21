/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import JobSubnav from 'nomad-ui/components/job-subnav';
import LoadingSpinner from 'nomad-ui/components/loading-spinner';

<template>
  <JobSubnav @job={{@controller.job}} />
  <section class="section has-text-centered"><LoadingSpinner /></section>
</template>
