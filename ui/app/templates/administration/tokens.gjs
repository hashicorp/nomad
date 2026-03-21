/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import { pageTitle } from 'ember-page-title';

<template>
  {{pageTitle "Tokens"}}
  <Breadcrumb
    @crumb={{hash label="Tokens" args=(array "administration.tokens")}}
  />
  {{outlet}}
</template>
