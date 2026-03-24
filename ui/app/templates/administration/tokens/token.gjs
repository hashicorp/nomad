/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import { pageTitle } from 'ember-page-title';
import can from 'ember-can/helpers/can';
import perform from 'ember-concurrency/helpers/perform';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import TokenEditor from 'nomad-ui/components/token-editor';
import TwoStepButton from 'nomad-ui/components/two-step-button';
import { HdsPageHeader } from '@hashicorp/design-system-components/components';

<template>
  <Breadcrumb
    @crumb={{hash
      label=@controller.activeToken.name
      args=(array "administration.tokens.token" @controller.activeToken.id)
    }}
  />
  {{pageTitle "Token"}}

  <section class="section">
    <HdsPageHeader as |PH|>
      <PH.Title data-test-title>
        Edit Token
      </PH.Title>
      {{#if (can "destroy token")}}
        <PH.Actions>
          <TwoStepButton
            data-test-delete-token
            @alignRight={{true}}
            @idleText="Delete Token"
            @cancelText="Cancel"
            @confirmText="Yes, Delete Token"
            @confirmationMessage="Are you sure?"
            @awaitingConfirmation={{@controller.deleteToken.isRunning}}
            @disabled={{@controller.deleteToken.isRunning}}
            @onConfirm={{perform @controller.deleteToken}}
          />
        </PH.Actions>
      {{/if}}
    </HdsPageHeader>

    <TokenEditor
      @token={{@controller.activeToken}}
      @policies={{@controller.policies}}
      @roles={{@controller.roles}}
    />
  </section>
</template>
