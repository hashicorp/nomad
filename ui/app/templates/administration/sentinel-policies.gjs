/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import { pageTitle } from 'ember-page-title';

<template>
  <Breadcrumb
    @crumb={{hash
      label="Sentinel Policies"
      args=(array "administration.sentinel-policies.index")
    }}
  />
  {{pageTitle "Sentinel Policies"}}
  {{outlet}}
</template>
