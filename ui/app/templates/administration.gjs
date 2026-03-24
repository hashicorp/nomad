/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import PageLayout from 'nomad-ui/components/page-layout';
import AdministrationSubnav from 'nomad-ui/components/administration-subnav';
import { pageTitle } from 'ember-page-title';

<template>
  {{pageTitle "Administration"}}

  <Breadcrumb
    @crumb={{hash label="Administration" args=(array "administration")}}
  />
  <PageLayout>
    <AdministrationSubnav @client={{@model}} />
    {{outlet}}
  </PageLayout>
</template>
