/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import { pageTitle } from 'ember-page-title';
import { HdsPageHeader } from '@hashicorp/design-system-components/components';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import TokenEditor from 'nomad-ui/components/token-editor';

<template>
  <Breadcrumb
    @crumb={{hash label="New" args=(array "administration.tokens.new")}}
  />
  {{pageTitle "Create Token"}}
  <section class="section">
    <HdsPageHeader as |PH|>
      <PH.Title>Create Token</PH.Title>
    </HdsPageHeader>
    <TokenEditor
      @token={{@model.token}}
      @roles={{@model.roles}}
      @policies={{@model.policies}}
    />
  </section>
</template>
