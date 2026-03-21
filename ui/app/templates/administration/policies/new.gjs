/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import { pageTitle } from 'ember-page-title';
import { HdsPageHeader } from '@hashicorp/design-system-components/components';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import PolicyEditor from 'nomad-ui/components/policy-editor';

<template>
  <Breadcrumb
    @crumb={{hash label="New" args=(array "administration.policies.new")}}
  />
  {{pageTitle "Create Policy"}}
  <section class="section">
    <HdsPageHeader as |PH|>
      <PH.Title>Create Policy</PH.Title>
    </HdsPageHeader>
    <PolicyEditor @policy={{@model}} />
  </section>
</template>
