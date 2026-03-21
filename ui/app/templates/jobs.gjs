/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import PageLayout from 'nomad-ui/components/page-layout';

<template>
  <Breadcrumb @crumb={{hash label="Jobs" args=(array "jobs.index")}} />
  <PageLayout>
    {{outlet}}
  </PageLayout>
</template>
