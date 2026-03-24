/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import { pageTitle } from 'ember-page-title';
import can from 'ember-can/helpers/can';
import perform from 'ember-concurrency/helpers/perform';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import SentinelPolicyEditor from 'nomad-ui/components/sentinel-policy-editor';
import TwoStepButton from 'nomad-ui/components/two-step-button';
import { HdsPageHeader } from '@hashicorp/design-system-components/components';

<template>
  <Breadcrumb
    @crumb={{hash
      label=@model.name
      args=(array "administration.sentinel-policies.policy" @model.name)
    }}
  />
  {{pageTitle "Sentinel Policy:" @model.name}}

  <section class="section">
    <HdsPageHeader class="variable-title" as |PH|>
      <PH.Title>{{@model.name}}</PH.Title>
      {{#if (can "destroy sentinel-policy")}}
        <PH.Actions>
          <div>
            <TwoStepButton
              data-test-delete-policy
              @alignRight={{true}}
              @idleText="Delete Sentinel Policy"
              @cancelText="Cancel"
              @confirmText="Yes, Delete Policy"
              @confirmationMessage="Are you sure?"
              @awaitingConfirmation={{@controller.deletePolicy.isRunning}}
              @disabled={{@controller.deletePolicy.isRunning}}
              @onConfirm={{perform @controller.deletePolicy}}
            />
          </div>
        </PH.Actions>
      {{/if}}
    </HdsPageHeader>

    <SentinelPolicyEditor @policy={{@model}} />
  </section>
</template>
