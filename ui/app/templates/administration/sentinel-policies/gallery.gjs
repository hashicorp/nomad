/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { hash, array } from '@ember/helper';
import { on } from '@ember/modifier';
import { pageTitle } from 'ember-page-title';
import eq from 'ember-truth-helpers/helpers/eq';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import {
  HdsButton,
  HdsButtonSet,
  HdsFormRadioCardGroup,
  HdsPageHeader,
} from '@hashicorp/design-system-components/components';

<template>
  <Breadcrumb
    @crumb={{hash
      label="Gallery"
      args=(array "administration.sentinel-policies.gallery")
    }}
  />
  {{pageTitle "Sentinel Policy Gallery"}}

  <section class="section">
    <HdsPageHeader class="variable-title" as |PH|>
      <PH.Title>Choose a Template</PH.Title>
      <PH.Description>
        Select a policy template below. You will have an opportunity to modify
        the policy before it is submitted.
      </PH.Description>
    </HdsPageHeader>

    <main class="radio-group" data-test-template-list>
      <HdsFormRadioCardGroup as |G|>
        <G.Legend>Select a Template</G.Legend>
        {{#each @controller.templates as |template|}}
          <G.RadioCard
            class="form-container"
            @layout="fixed"
            @maxWidth="30%"
            @checked={{eq template.name @controller.selectedTemplate}}
            id={{template.name}}
            {{on "change" @controller.onChange}}
            as |R|
          >
            <R.Label
              data-test-template-card={{template.name}}
              data-test-template-label
            >{{template.displayName}}</R.Label>
            <R.Description
              data-test-template-description
            >{{template.description}}</R.Description>
          </G.RadioCard>
        {{/each}}
      </HdsFormRadioCardGroup>
    </main>

    <footer class="buttonset">
      <HdsButtonSet class="button-group">
        <HdsButton
          @text="Apply"
          @route="administration.sentinel-policies.new"
          @query={{hash template=@controller.selectedTemplate}}
          disabled={{eq @controller.selectedTemplate null}}
          data-test-apply
        />
        <HdsButton
          @text="Cancel"
          @route="administration.sentinel-policies.new"
          @color="secondary"
          data-test-cancel
        />
      </HdsButtonSet>
    </footer>
  </section>
</template>
