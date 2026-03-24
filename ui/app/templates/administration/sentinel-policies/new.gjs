/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import { pageTitle } from 'ember-page-title';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import SentinelPolicyEditor from 'nomad-ui/components/sentinel-policy-editor';
import {
  HdsButton,
  HdsLinkInline,
  HdsPageHeader,
} from '@hashicorp/design-system-components/components';

<template>
  <Breadcrumb
    @crumb={{hash
      label="New"
      args=(array "administration.sentinel-policies.new")
    }}
  />
  {{pageTitle "Create a Policy"}}

  <section class="section">
    <HdsPageHeader class="variable-title" as |PH|>
      <PH.Title>Create Sentinel Policy</PH.Title>
      <PH.Description>
        Nomad integrates with
        <HdsLinkInline
          @icon="collections"
          @href="https://developer.hashicorp.com/nomad/docs/govern/sentinel"
        >HashiCorp Sentinel</HdsLinkInline>
        to allow operators to express policies as code and have those policies
        automatically enforced. This allows operators to define a "sandbox" and
        restrict actions to only those compliant with that policy.
      </PH.Description>
      <PH.Actions>
        <HdsButton
          @text="Start from a template"
          @color="secondary"
          @route="administration.sentinel-policies.gallery"
          data-test-choose-template
        />
      </PH.Actions>
    </HdsPageHeader>

    <SentinelPolicyEditor @policy={{@model}} />
  </section>
</template>
