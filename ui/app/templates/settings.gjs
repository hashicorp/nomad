/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import willDestroy from '@ember/render-modifiers/modifiers/will-destroy';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import PageLayout from 'nomad-ui/components/page-layout';

<template>
  <Breadcrumb @crumb={{hash label="Settings" args=(array "settings.tokens")}} />
  <PageLayout>
    <div
      class="tabs is-subnav"
      {{didInsert @controller.keyboard.registerNav type="subnav"}}
      {{willDestroy @controller.keyboard.unregisterSubnav}}
    >
      <ul>
        {{#if @controller.shouldShowProfileLink}}
          <li><LinkTo @route="settings.tokens" @activeClass="is-active">
              {{#if @controller.tokenRecord}}
                Profile
              {{else}}
                Sign In
              {{/if}}
            </LinkTo></li>
        {{/if}}
        <li><LinkTo
            @route="settings.user-settings"
            @activeClass="is-active"
          >User Settings</LinkTo></li>
      </ul>
    </div>
    {{outlet}}
  </PageLayout>
</template>
