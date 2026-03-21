/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import { pageTitle } from 'ember-page-title';

<template>
  {{pageTitle "Policies"}}
  <Breadcrumb
    @crumb={{hash label="Policies" args=(array "administration.policies")}}
  />
  {{outlet}}
</template>
